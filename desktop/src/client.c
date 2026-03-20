#include "solock-desktop.h"
#include <gio/gunixsocketaddress.h>
#include <sys/types.h>
#include <signal.h>
#include <unistd.h>
#include <string.h>

struct _SolockClient {
    char    *sock_path;
    char    *token;
    GPid     serve_pid;
    gboolean locked;
};

SolockClient *solock_client_new(void)
{
    SolockClient *c = g_new0(SolockClient, 1);
    c->locked = TRUE;
    return c;
}

void solock_client_free(SolockClient *c)
{
    g_free(c->sock_path);
    g_free(c->token);
    g_free(c);
}

static void cleanup_stale_socket(void)
{
    const char *home = g_get_home_dir();
    char *sock_path = g_build_filename(home, ".solock", "solock.sock", NULL);
    if (g_file_test(sock_path, G_FILE_TEST_EXISTS)) {
        unlink(sock_path);
    }
    g_free(sock_path);
}

gboolean solock_client_start_serve(SolockClient *c, GError **error)
{
    cleanup_stale_socket();

    gchar *argv[] = { (gchar *)SOLOCK_BINARY, "serve", NULL };
    gint stdout_fd, stderr_fd;

    if (!g_spawn_async_with_pipes(NULL, argv, NULL,
                                   G_SPAWN_SEARCH_PATH,
                                   NULL, NULL, &c->serve_pid,
                                   NULL, &stdout_fd, &stderr_fd, error)) {
        return FALSE;
    }

    GIOChannel *channel = g_io_channel_unix_new(stdout_fd);
    gchar *line1 = NULL, *line2 = NULL;
    gsize len;
    GIOStatus s1, s2;

    s1 = g_io_channel_read_line(channel, &line1, &len, NULL, NULL);
    s2 = g_io_channel_read_line(channel, &line2, &len, NULL, NULL);
    g_io_channel_unref(channel);
    close(stdout_fd);

    if (s1 != G_IO_STATUS_NORMAL || s2 != G_IO_STATUS_NORMAL || !line1 || !line2) {
        GIOChannel *err_ch = g_io_channel_unix_new(stderr_fd);
        gchar *err_line = NULL;
        g_io_channel_read_line(err_ch, &err_line, NULL, NULL, NULL);
        g_io_channel_unref(err_ch);
        close(stderr_fd);

        if (err_line) {
            g_set_error(error, G_IO_ERROR, G_IO_ERROR_FAILED,
                        "solock serve failed: %s", g_strstrip(err_line));
            g_free(err_line);
        } else {
            g_set_error(error, G_IO_ERROR, G_IO_ERROR_FAILED,
                        "solock serve failed to start");
        }
        g_free(line1);
        g_free(line2);
        return FALSE;
    }
    close(stderr_fd);

    c->sock_path = g_strstrip(g_strdup(line1));
    c->token = g_strstrip(g_strdup(line2));
    g_free(line1);
    g_free(line2);

    return TRUE;
}

void solock_client_stop_serve(SolockClient *c)
{
    if (c->serve_pid > 0) {
        kill(c->serve_pid, SIGTERM);
        g_spawn_close_pid(c->serve_pid);
        c->serve_pid = 0;
    }
}

gboolean solock_client_is_connected(SolockClient *c)
{
    return c->sock_path != NULL;
}

static GSocketConnection *connect_to_socket(SolockClient *c, GError **error)
{
    GSocketAddress *addr = g_unix_socket_address_new(c->sock_path);
    GSocketClient *socket_client = g_socket_client_new();
    GSocketConnection *conn = g_socket_client_connect(socket_client,
                                                       G_SOCKET_CONNECTABLE(addr),
                                                       NULL, error);
    g_object_unref(addr);
    g_object_unref(socket_client);
    return conn;
}

JsonNode *solock_client_call(SolockClient *c, const char *method, JsonNode *params, GError **error)
{
    if (!c->sock_path) {
        g_set_error(error, G_IO_ERROR, G_IO_ERROR_NOT_CONNECTED, "not connected");
        return NULL;
    }

    JsonBuilder *builder = json_builder_new();
    json_builder_begin_object(builder);
    json_builder_set_member_name(builder, "jsonrpc");
    json_builder_add_string_value(builder, "2.0");
    json_builder_set_member_name(builder, "method");
    json_builder_add_string_value(builder, method);

    json_builder_set_member_name(builder, "params");
    if (params) {
        JsonObject *obj = json_node_get_object(params);
        json_builder_begin_object(builder);
        GList *members = json_object_get_members(obj);
        for (GList *l = members; l; l = l->next) {
            const char *name = l->data;
            json_builder_set_member_name(builder, name);
            JsonNode *val = json_object_get_member(obj, name);
            json_builder_add_value(builder, json_node_copy(val));
        }
        g_list_free(members);
        json_builder_set_member_name(builder, "token");
        json_builder_add_string_value(builder, c->token);
        json_builder_end_object(builder);
    } else {
        json_builder_begin_object(builder);
        json_builder_set_member_name(builder, "token");
        json_builder_add_string_value(builder, c->token);
        json_builder_end_object(builder);
    }

    json_builder_set_member_name(builder, "id");
    json_builder_add_int_value(builder, 1);
    json_builder_end_object(builder);

    JsonGenerator *gen = json_generator_new();
    JsonNode *root = json_builder_get_root(builder);
    json_generator_set_root(gen, root);
    gchar *request_str = json_generator_to_data(gen, NULL);
    g_object_unref(gen);
    json_node_unref(root);
    g_object_unref(builder);

    gchar *line = g_strconcat(request_str, "\n", NULL);
    g_free(request_str);

    GSocketConnection *conn = connect_to_socket(c, error);
    if (!conn) {
        g_free(line);
        return NULL;
    }

    GOutputStream *out = g_io_stream_get_output_stream(G_IO_STREAM(conn));
    g_output_stream_write_all(out, line, strlen(line), NULL, NULL, error);
    g_output_stream_flush(out, NULL, NULL);
    g_free(line);

    GInputStream *in = g_io_stream_get_input_stream(G_IO_STREAM(conn));
    GDataInputStream *data_in = g_data_input_stream_new(in);
    gchar *response_line = g_data_input_stream_read_line(data_in, NULL, NULL, error);
    g_object_unref(data_in);
    g_io_stream_close(G_IO_STREAM(conn), NULL, NULL);
    g_object_unref(conn);

    if (!response_line) {
        g_set_error(error, G_IO_ERROR, G_IO_ERROR_FAILED, "empty response");
        return NULL;
    }

    JsonParser *parser = json_parser_new();
    if (!json_parser_load_from_data(parser, response_line, -1, error)) {
        g_free(response_line);
        g_object_unref(parser);
        return NULL;
    }
    g_free(response_line);

    JsonNode *resp = json_node_copy(json_parser_get_root(parser));
    g_object_unref(parser);

    JsonObject *resp_obj = json_node_get_object(resp);
    if (json_object_has_member(resp_obj, "error")) {
        JsonObject *err_obj = json_object_get_object_member(resp_obj, "error");
        const char *msg = json_object_get_string_member(err_obj, "message");
        g_set_error(error, G_IO_ERROR, G_IO_ERROR_FAILED, "%s", msg);
        json_node_unref(resp);
        return NULL;
    }

    if (json_object_has_member(resp_obj, "result")) {
        JsonNode *result = json_node_copy(json_object_get_member(resp_obj, "result"));
        json_node_unref(resp);
        return result;
    }

    json_node_unref(resp);
    return NULL;
}

gboolean solock_client_unlock(SolockClient *c, const char *password, int timeout_minutes, GError **error)
{
    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "password");
    json_builder_add_string_value(b, password);
    json_builder_set_member_name(b, "timeout_minutes");
    json_builder_add_int_value(b, timeout_minutes);
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);

    JsonNode *result = solock_client_call(c, "unlock", params, error);
    json_node_unref(params);
    g_object_unref(b);

    if (result) {
        c->locked = FALSE;
        json_node_unref(result);
        return TRUE;
    }
    return FALSE;
}

void solock_client_lock(SolockClient *c)
{
    solock_client_call(c, "lock", NULL, NULL);
    c->locked = TRUE;
}

gboolean solock_client_is_locked(SolockClient *c)
{
    return c->locked;
}

JsonNode *solock_client_list_entries(SolockClient *c, GError **error)
{
    return solock_client_call(c, "list_entries", NULL, error);
}

JsonNode *solock_client_search_entries(SolockClient *c, const char *query, GError **error)
{
    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "query");
    json_builder_add_string_value(b, query);
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);

    JsonNode *result = solock_client_call(c, "search_entries", params, error);
    json_node_unref(params);
    g_object_unref(b);
    return result;
}

JsonNode *solock_client_get_entry(SolockClient *c, const char *id, GError **error)
{
    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "id");
    json_builder_add_string_value(b, id);
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);

    JsonNode *result = solock_client_call(c, "get_entry", params, error);
    json_node_unref(params);
    g_object_unref(b);
    return result;
}

JsonNode *solock_client_get_dashboard(SolockClient *c, GError **error)
{
    return solock_client_call(c, "get_dashboard", NULL, error);
}

gboolean solock_client_add_entry(SolockClient *c, const char *type, const char *name, JsonNode *fields, GError **error)
{
    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "type");
    json_builder_add_string_value(b, type);
    json_builder_set_member_name(b, "name");
    json_builder_add_string_value(b, name);
    json_builder_set_member_name(b, "fields");
    json_builder_add_value(b, json_node_copy(fields));
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);

    JsonNode *result = solock_client_call(c, "add_entry", params, error);
    json_node_unref(params);
    g_object_unref(b);
    if (result) { json_node_unref(result); return TRUE; }
    return FALSE;
}

gboolean solock_client_update_entry(SolockClient *c, const char *id, const char *name, JsonNode *fields, GError **error)
{
    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "id");
    json_builder_add_string_value(b, id);
    if (name) {
        json_builder_set_member_name(b, "name");
        json_builder_add_string_value(b, name);
    }
    if (fields) {
        json_builder_set_member_name(b, "fields");
        json_builder_add_value(b, json_node_copy(fields));
    }
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);

    JsonNode *result = solock_client_call(c, "update_entry", params, error);
    json_node_unref(params);
    g_object_unref(b);
    if (result) { json_node_unref(result); return TRUE; }
    return FALSE;
}

gboolean solock_client_delete_entry(SolockClient *c, const char *id, GError **error)
{
    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "id");
    json_builder_add_string_value(b, id);
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);

    JsonNode *result = solock_client_call(c, "delete_entry", params, error);
    json_node_unref(params);
    g_object_unref(b);
    if (result) { json_node_unref(result); return TRUE; }
    return FALSE;
}

gboolean solock_client_sync(SolockClient *c, GError **error)
{
    JsonNode *result = solock_client_call(c, "sync", NULL, error);
    if (result) { json_node_unref(result); return TRUE; }
    return FALSE;
}

gboolean solock_client_deploy_program(SolockClient *c, GError **error)
{
    JsonNode *result = solock_client_call(c, "deploy_program", NULL, error);
    if (result) { json_node_unref(result); return TRUE; }
    return FALSE;
}

gboolean solock_client_init_vault(SolockClient *c, GError **error)
{
    JsonNode *result = solock_client_call(c, "initialize_vault", NULL, error);
    if (result) { json_node_unref(result); return TRUE; }
    return FALSE;
}

char *solock_client_generate_password(SolockClient *c, int length, gboolean uppercase, gboolean digits, gboolean special, GError **error)
{
    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "length");
    json_builder_add_int_value(b, length);
    json_builder_set_member_name(b, "uppercase");
    json_builder_add_boolean_value(b, uppercase);
    json_builder_set_member_name(b, "digits");
    json_builder_add_boolean_value(b, digits);
    json_builder_set_member_name(b, "special");
    json_builder_add_boolean_value(b, special);
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);

    JsonNode *result = solock_client_call(c, "generate_password", params, error);
    json_node_unref(params);
    g_object_unref(b);

    if (!result) return NULL;
    JsonObject *obj = json_node_get_object(result);
    char *pw = g_strdup(json_object_get_string_member(obj, "password"));
    json_node_unref(result);
    return pw;
}

JsonNode *solock_client_generate_totp(SolockClient *c, const char *secret, int digits, int period, GError **error)
{
    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "secret");
    json_builder_add_string_value(b, secret);
    json_builder_set_member_name(b, "digits");
    json_builder_add_int_value(b, digits);
    json_builder_set_member_name(b, "period");
    json_builder_add_int_value(b, period);
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);

    JsonNode *result = solock_client_call(c, "generate_totp", params, error);
    json_node_unref(params);
    g_object_unref(b);
    return result;
}

JsonNode *solock_client_status(SolockClient *c, GError **error)
{
    return solock_client_call(c, "status", NULL, error);
}

void solock_client_shutdown(SolockClient *c)
{
    solock_client_call(c, "shutdown", NULL, NULL);
}

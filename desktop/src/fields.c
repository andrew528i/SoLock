#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);

static const char *FIELD_LABELS = "asdfgh";

typedef struct {
    SolockApp *app;
    JsonNode  *entry;
    guint      totp_timer;
    GtkWidget *totp_label;
    GtkWidget *totp_bar;
} FieldsData;

static void on_field_paste(SolockApp *app, const char *value)
{
    SolockConfig *config = solock_app_get_config(app);
    const char *method = solock_config_get_paste_method(config);

    GtkWidget *popup = solock_app_get_popup(app);
    solock_popup_hide(popup);

    if (g_strcmp0(method, "wtype") == 0 && solock_wtype_available()) {
        solock_wtype_type(value, NULL);
    } else {
        int clear = solock_config_get_clipboard_clear_seconds(config);
        solock_clipboard_copy(value, clear, NULL);
    }
}

static gboolean update_totp(gpointer data)
{
    FieldsData *fd = data;
    JsonObject *obj = json_node_get_object(fd->entry);
    const char *type = json_object_get_string_member(obj, "type");

    const char *secret = NULL;
    if (g_strcmp0(type, "password") == 0) {
        JsonObject *fields = json_object_get_object_member(obj, "fields");
        if (json_object_has_member(fields, "totp_secret"))
            secret = json_object_get_string_member(fields, "totp_secret");
    } else if (g_strcmp0(type, "totp") == 0) {
        JsonObject *fields = json_object_get_object_member(obj, "fields");
        if (json_object_has_member(fields, "secret"))
            secret = json_object_get_string_member(fields, "secret");
    }

    if (!secret || !*secret) return G_SOURCE_CONTINUE;

    SolockClient *client = solock_app_get_client(fd->app);
    JsonNode *result = solock_client_generate_totp(client, secret, 0, 0, NULL);
    if (!result) return G_SOURCE_CONTINUE;

    JsonObject *res = json_node_get_object(result);
    const char *code = json_object_get_string_member(res, "code");
    gint64 remaining = json_object_get_int_member(res, "remaining");

    char *label_text = g_strdup_printf("%s  %llds", code, (long long)remaining);
    gtk_label_set_text(GTK_LABEL(fd->totp_label), label_text);
    g_free(label_text);

    gtk_level_bar_set_value(GTK_LEVEL_BAR(fd->totp_bar), (double)remaining / 30.0);

    json_node_unref(result);
    return G_SOURCE_CONTINUE;
}

GtkWidget *solock_fields_view_new(SolockApp *app, JsonNode *entry)
{
    JsonObject *obj = json_node_get_object(entry);
    const char *name = json_object_get_string_member(obj, "name");
    JsonObject *fields = json_object_get_object_member(obj, "fields");
    gboolean has_totp = json_object_get_boolean_member_with_default(obj, "has_totp", FALSE);

    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 8);
    gtk_widget_set_margin_start(box, 12);
    gtk_widget_set_margin_end(box, 12);
    gtk_widget_set_margin_top(box, 12);
    gtk_widget_set_margin_bottom(box, 12);

    GtkWidget *title = gtk_label_new(name);
    gtk_widget_add_css_class(title, "title-4");
    gtk_label_set_xalign(GTK_LABEL(title), 0);
    gtk_box_append(GTK_BOX(box), title);

    GList *members = json_object_get_members(fields);
    int idx = 0;
    for (GList *l = members; l; l = l->next, idx++) {
        const char *key = l->data;
        const char *value = json_object_get_string_member(fields, key);
        if (!value || !*value) continue;

        GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
        gtk_widget_set_margin_top(row, 2);

        char lbl[4];
        if (idx < (int)strlen(FIELD_LABELS))
            g_snprintf(lbl, sizeof(lbl), "%c", FIELD_LABELS[idx]);
        else
            g_snprintf(lbl, sizeof(lbl), "%d", idx);

        GtkWidget *hint = gtk_label_new(lbl);
        gtk_widget_add_css_class(hint, "label-hint");
        gtk_widget_add_css_class(hint, "monospace");
        gtk_box_append(GTK_BOX(row), hint);

        GtkWidget *key_label = gtk_label_new(key);
        gtk_widget_add_css_class(key_label, "dim-label");
        gtk_widget_set_size_request(key_label, 100, -1);
        gtk_label_set_xalign(GTK_LABEL(key_label), 0);
        gtk_box_append(GTK_BOX(row), key_label);

        gtk_box_append(GTK_BOX(box), row);
    }
    g_list_free(members);

    if (has_totp) {
        GtkWidget *sep = gtk_separator_new(GTK_ORIENTATION_HORIZONTAL);
        gtk_box_append(GTK_BOX(box), sep);

        GtkWidget *totp_label = gtk_label_new("------");
        gtk_widget_add_css_class(totp_label, "title-2");
        gtk_widget_add_css_class(totp_label, "monospace");
        gtk_box_append(GTK_BOX(box), totp_label);

        GtkWidget *totp_bar = gtk_level_bar_new_for_interval(0.0, 1.0);
        gtk_level_bar_set_value(GTK_LEVEL_BAR(totp_bar), 1.0);
        gtk_box_append(GTK_BOX(box), totp_bar);

        FieldsData *fd = g_new0(FieldsData, 1);
        fd->app = app;
        fd->entry = json_node_copy(entry);
        fd->totp_label = totp_label;
        fd->totp_bar = totp_bar;

        update_totp(fd);
        fd->totp_timer = g_timeout_add_seconds(1, update_totp, fd);
    }

    return box;
}

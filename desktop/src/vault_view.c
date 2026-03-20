#include "solock-desktop.h"
#include <string.h>

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);

static const char *icon_for_type(const char *type)
{
    if (g_strcmp0(type, "password") == 0) return "dialog-password-symbolic";
    if (g_strcmp0(type, "card") == 0)     return "credit-card-symbolic";
    if (g_strcmp0(type, "note") == 0)     return "document-text-symbolic";
    if (g_strcmp0(type, "totp") == 0)     return "fingerprint-symbolic";
    return "dialog-password-symbolic";
}

static const char *type_display_name(const char *type)
{
    if (g_strcmp0(type, "password") == 0) return "Password";
    if (g_strcmp0(type, "card") == 0)     return "Card";
    if (g_strcmp0(type, "note") == 0)     return "Note";
    if (g_strcmp0(type, "totp") == 0)     return "TOTP";
    return type;
}

static gboolean is_sensitive_field(const char *key)
{
    return g_strcmp0(key, "password") == 0 ||
           g_strcmp0(key, "cvv") == 0 ||
           g_strcmp0(key, "number") == 0 ||
           g_strcmp0(key, "totp_secret") == 0 ||
           g_strcmp0(key, "secret") == 0;
}

static const char *human_label(const char *key)
{
    if (g_strcmp0(key, "password") == 0)   return "Password";
    if (g_strcmp0(key, "username") == 0)   return "Username";
    if (g_strcmp0(key, "site") == 0)       return "Website";
    if (g_strcmp0(key, "number") == 0)     return "Card Number";
    if (g_strcmp0(key, "cvv") == 0)        return "CVV";
    if (g_strcmp0(key, "expiry") == 0)     return "Expiry";
    if (g_strcmp0(key, "cardholder") == 0) return "Cardholder";
    if (g_strcmp0(key, "content") == 0)    return "Content";
    if (g_strcmp0(key, "notes") == 0)      return "Notes";
    if (g_strcmp0(key, "secret") == 0)     return "Secret";
    if (g_strcmp0(key, "totp_secret") == 0) return "TOTP Secret";
    return key;
}

/* entry types and their fields */
typedef struct {
    const char *type;
    const char *display;
    const char **fields;
} EntryTypeDef;

static const char *password_fields[] = { "site", "username", "password", NULL };
static const char *note_fields[]     = { "content", NULL };
static const char *card_fields[]     = { "cardholder", "number", "expiry", "cvv", NULL };
static const char *totp_fields[]     = { "secret", NULL };

static const EntryTypeDef entry_types[] = {
    { "password", "Password", password_fields },
    { "note",     "Note",     note_fields },
    { "card",     "Card",     card_fields },
    { "totp",     "TOTP",     totp_fields },
};
static const int entry_types_count = 4;

typedef struct {
    SolockApp *app;
    GtkWidget *list_box;
    GtkWidget *detail_stack;
    GtkWidget *search_entry;
    JsonNode  *entries;

    /* detail panel widgets */
    GtkWidget *detail_empty;
    GtkWidget *detail_view;
    GtkWidget *detail_name_label;
    GtkWidget *detail_type_label;
    GtkWidget *detail_fields_box;
    GtkWidget *detail_edit_btn;
    GtkWidget *detail_delete_btn;
    GtkWidget *detail_action_bar;
    char      *selected_id;

    /* detail totp */
    GtkWidget *detail_totp_code;
    GtkWidget *detail_totp_bar;
    guint      detail_totp_timer;
    char      *detail_totp_secret;

    /* stats labels */
    GtkWidget *stats_total;
    GtkWidget *stats_passwords;
    GtkWidget *stats_cards;
    GtkWidget *stats_notes;
    GtkWidget *stats_totp;

    /* add form */
    GtkWidget *add_stack;
    GtkWidget *add_type_dropdown;
    GtkWidget *add_name_entry;
    GtkWidget *add_fields_box;
    GtkWidget *add_save_btn;

    /* edit state */
    gboolean   editing;
    GtkWidget *edit_name_entry;
    GtkWidget *edit_fields_box;
    GtkWidget *edit_save_btn;
    GtkWidget *edit_cancel_btn;
} VaultData;

static void vault_refresh_entries(VaultData *vd);
static void vault_show_detail(VaultData *vd, JsonObject *obj);
static void vault_clear_detail(VaultData *vd);
static void vault_stop_detail_totp(VaultData *vd);

static void on_copy_field_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    const char *text = data;
    if (text && *text)
        solock_clipboard_copy(text, 0, NULL);
}

static void on_copy_totp_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    GtkWidget *code_label = data;
    const char *text = gtk_label_get_text(GTK_LABEL(code_label));
    if (!text || !*text) return;

    char *clean = g_strdup(text);
    char *dst = clean;
    for (const char *src = text; *src; src++) {
        if (*src != ' ')
            *dst++ = *src;
    }
    *dst = '\0';
    solock_clipboard_copy(clean, 0, NULL);
    g_free(clean);
}

static void vault_clear_container(GtkWidget *container)
{
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(container)) != NULL)
        gtk_box_remove(GTK_BOX(container), child);
}

/* list */

static void on_entry_row_activated(GtkListBox *list_box, GtkListBoxRow *row, gpointer data)
{
    (void)list_box;
    VaultData *vd = data;
    if (!row || !vd->entries) return;

    int idx = gtk_list_box_row_get_index(row);
    JsonArray *arr = json_node_get_array(vd->entries);
    if ((guint)idx >= json_array_get_length(arr)) return;

    JsonObject *obj = json_array_get_object_element(arr, idx);
    vault_show_detail(vd, obj);
}

static void on_search_changed(GtkEditable *editable, gpointer data)
{
    (void)editable;
    VaultData *vd = data;
    vault_refresh_entries(vd);
}

static void vault_update_stats(VaultData *vd)
{
    int total = 0, passwords = 0, cards = 0, notes = 0, totp = 0;

    if (vd->entries && JSON_NODE_TYPE(vd->entries) == JSON_NODE_ARRAY) {
        JsonArray *arr = json_node_get_array(vd->entries);
        total = (int)json_array_get_length(arr);
        for (guint i = 0; i < (guint)total; i++) {
            JsonObject *obj = json_array_get_object_element(arr, i);
            const char *type = json_object_get_string_member(obj, "type");
            if (g_strcmp0(type, "password") == 0) passwords++;
            else if (g_strcmp0(type, "card") == 0) cards++;
            else if (g_strcmp0(type, "note") == 0) notes++;
            if (g_strcmp0(type, "totp") == 0 ||
                json_object_get_boolean_member_with_default(obj, "has_totp", FALSE))
                totp++;
        }
    }

    char buf[32];
    g_snprintf(buf, sizeof(buf), "%d", total);
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_total), buf);
    g_snprintf(buf, sizeof(buf), "%d", passwords);
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_passwords), buf);
    g_snprintf(buf, sizeof(buf), "%d", cards);
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_cards), buf);
    g_snprintf(buf, sizeof(buf), "%d", notes);
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_notes), buf);
    g_snprintf(buf, sizeof(buf), "%d", totp);
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_totp), buf);
}

static void vault_refresh_entries(VaultData *vd)
{
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(vd->list_box)) != NULL)
        gtk_list_box_remove(GTK_LIST_BOX(vd->list_box), child);

    if (vd->entries) {
        json_node_unref(vd->entries);
        vd->entries = NULL;
    }

    SolockClient *client = solock_app_get_client(vd->app);
    if (solock_client_is_locked(client)) return;

    GError *error = NULL;
    const char *query = gtk_editable_get_text(GTK_EDITABLE(vd->search_entry));

    if (query && *query)
        vd->entries = solock_client_search_entries(client, query, &error);
    else
        vd->entries = solock_client_list_entries(client, &error);

    if (error) {
        g_warning("Failed to list entries: %s", error->message);
        g_error_free(error);
    }

    if (!vd->entries || JSON_NODE_TYPE(vd->entries) != JSON_NODE_ARRAY) {
        vault_update_stats(vd);
        return;
    }

    JsonArray *arr = json_node_get_array(vd->entries);
    guint len = json_array_get_length(arr);

    for (guint i = 0; i < len; i++) {
        JsonObject *obj = json_array_get_object_element(arr, i);
        const char *name = json_object_get_string_member(obj, "name");
        const char *type = json_object_get_string_member(obj, "type");

        JsonObject *fields = json_object_get_object_member(obj, "fields");
        const char *subtitle = NULL;
        if (fields) {
            if (json_object_has_member(fields, "site"))
                subtitle = json_object_get_string_member(fields, "site");
            else if (json_object_has_member(fields, "username"))
                subtitle = json_object_get_string_member(fields, "username");
            else if (json_object_has_member(fields, "cardholder"))
                subtitle = json_object_get_string_member(fields, "cardholder");
        }

        GtkWidget *row_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 10);
        gtk_widget_set_margin_start(row_box, 8);
        gtk_widget_set_margin_end(row_box, 8);
        gtk_widget_set_margin_top(row_box, 6);
        gtk_widget_set_margin_bottom(row_box, 6);
        gtk_widget_set_size_request(row_box, -1, 40);

        GtkWidget *icon = gtk_image_new_from_icon_name(icon_for_type(type));
        gtk_image_set_pixel_size(GTK_IMAGE(icon), 18);
        gtk_widget_add_css_class(icon, "dim-label");
        gtk_box_append(GTK_BOX(row_box), icon);

        GtkWidget *text_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
        gtk_widget_set_hexpand(text_box, TRUE);
        if (!subtitle || !*subtitle)
            gtk_widget_set_valign(text_box, GTK_ALIGN_CENTER);

        GtkWidget *name_label = gtk_label_new(name);
        gtk_label_set_xalign(GTK_LABEL(name_label), 0);
        gtk_label_set_ellipsize(GTK_LABEL(name_label), PANGO_ELLIPSIZE_END);
        gtk_box_append(GTK_BOX(text_box), name_label);

        if (subtitle && *subtitle) {
            GtkWidget *sub_label = gtk_label_new(subtitle);
            gtk_widget_add_css_class(sub_label, "dim-label");
            gtk_widget_add_css_class(sub_label, "caption");
            gtk_label_set_xalign(GTK_LABEL(sub_label), 0);
            gtk_label_set_ellipsize(GTK_LABEL(sub_label), PANGO_ELLIPSIZE_END);
            gtk_box_append(GTK_BOX(text_box), sub_label);
        }

        gtk_box_append(GTK_BOX(row_box), text_box);

        GtkWidget *type_badge = gtk_label_new(type_display_name(type));
        gtk_widget_add_css_class(type_badge, "dim-label");
        gtk_widget_add_css_class(type_badge, "caption");
        gtk_widget_set_valign(type_badge, GTK_ALIGN_CENTER);
        gtk_box_append(GTK_BOX(row_box), type_badge);

        gtk_list_box_append(GTK_LIST_BOX(vd->list_box), row_box);
    }

    vault_update_stats(vd);
}

/* detail view */

static void vault_clear_detail(VaultData *vd)
{
    vault_stop_detail_totp(vd);
    g_free(vd->selected_id);
    vd->selected_id = NULL;
    vd->editing = FALSE;
    gtk_stack_set_visible_child_name(GTK_STACK(vd->detail_stack), "empty");
}

static void vault_stop_detail_totp(VaultData *vd)
{
    if (vd->detail_totp_timer > 0) {
        g_source_remove(vd->detail_totp_timer);
        vd->detail_totp_timer = 0;
    }
    g_free(vd->detail_totp_secret);
    vd->detail_totp_secret = NULL;
    vd->detail_totp_code = NULL;
    vd->detail_totp_bar = NULL;
}

static gboolean vault_update_totp(gpointer data)
{
    VaultData *vd = data;
    if (!vd->detail_totp_secret || !vd->detail_totp_code) return G_SOURCE_REMOVE;

    SolockClient *client = solock_app_get_client(vd->app);
    JsonNode *result = solock_client_generate_totp(client, vd->detail_totp_secret, 0, 0, NULL);
    if (!result) return G_SOURCE_CONTINUE;

    JsonObject *res = json_node_get_object(result);
    const char *code = json_object_get_string_member(res, "code");
    gint64 remaining = json_object_get_int_member(res, "remaining");

    gsize code_len = strlen(code);
    char *formatted = NULL;
    if (code_len == 6)
        formatted = g_strdup_printf("%.3s %.3s", code, code + 3);
    else
        formatted = g_strdup(code);
    gtk_label_set_text(GTK_LABEL(vd->detail_totp_code), formatted);
    g_free(formatted);

    if (vd->detail_totp_bar) {
        gtk_level_bar_set_value(GTK_LEVEL_BAR(vd->detail_totp_bar), (double)remaining / 30.0);

        gtk_widget_remove_css_class(vd->detail_totp_bar, "totp-ok");
        gtk_widget_remove_css_class(vd->detail_totp_bar, "totp-warn");
        gtk_widget_remove_css_class(vd->detail_totp_bar, "totp-danger");
        if (remaining >= 10)
            gtk_widget_add_css_class(vd->detail_totp_bar, "totp-ok");
        else if (remaining >= 4)
            gtk_widget_add_css_class(vd->detail_totp_bar, "totp-warn");
        else
            gtk_widget_add_css_class(vd->detail_totp_bar, "totp-danger");
    }

    json_node_unref(result);
    return G_SOURCE_CONTINUE;
}

static const char *vault_get_totp_secret(JsonObject *obj)
{
    const char *type = json_object_get_string_member(obj, "type");
    JsonObject *fields = json_object_get_object_member(obj, "fields");
    if (!fields) return NULL;

    if (g_strcmp0(type, "password") == 0 && json_object_has_member(fields, "totp_secret"))
        return json_object_get_string_member(fields, "totp_secret");
    if (g_strcmp0(type, "totp") == 0 && json_object_has_member(fields, "secret"))
        return json_object_get_string_member(fields, "secret");
    return NULL;
}

static void vault_show_detail(VaultData *vd, JsonObject *obj)
{
    vd->editing = FALSE;
    vault_stop_detail_totp(vd);

    const char *id = json_object_get_string_member(obj, "id");
    const char *name = json_object_get_string_member(obj, "name");
    const char *type = json_object_get_string_member(obj, "type");
    JsonObject *fields = json_object_get_object_member(obj, "fields");

    g_free(vd->selected_id);
    vd->selected_id = g_strdup(id);

    gtk_label_set_text(GTK_LABEL(vd->detail_name_label), name);
    gtk_label_set_text(GTK_LABEL(vd->detail_type_label), type_display_name(type));

    vault_clear_container(vd->detail_fields_box);

    if (!fields) {
        gtk_widget_set_visible(vd->detail_action_bar, TRUE);
        gtk_stack_set_visible_child_name(GTK_STACK(vd->detail_stack), "detail");
        return;
    }

    GtkWidget *group = adw_preferences_group_new();
    gtk_widget_set_margin_start(group, 4);
    gtk_widget_set_margin_end(group, 4);

    GList *members = json_object_get_members(fields);
    for (GList *l = members; l; l = l->next) {
        const char *key = l->data;
        if (g_strcmp0(key, "totp_secret") == 0 || g_strcmp0(key, "secret") == 0)
            continue;

        const char *value = json_object_get_string_member(fields, key);
        if (!value) value = "";

        GtkWidget *row = adw_action_row_new();
        adw_preferences_row_set_title(ADW_PREFERENCES_ROW(row), human_label(key));
        gtk_widget_set_margin_top(row, 4);
        gtk_widget_set_margin_bottom(row, 4);
        gtk_widget_set_margin_start(row, 4);
        gtk_widget_set_margin_end(row, 4);

        if (is_sensitive_field(key)) {
            adw_action_row_set_subtitle(ADW_ACTION_ROW(row),
                "\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2");
        } else {
            adw_action_row_set_subtitle(ADW_ACTION_ROW(row), value);
            adw_action_row_set_subtitle_selectable(ADW_ACTION_ROW(row), TRUE);
        }

        GtkWidget *copy_btn = gtk_button_new_from_icon_name("edit-copy-symbolic");
        gtk_widget_add_css_class(copy_btn, "flat");
        gtk_widget_set_valign(copy_btn, GTK_ALIGN_CENTER);
        char *value_copy = g_strdup(value);
        g_signal_connect_data(copy_btn, "clicked", G_CALLBACK(on_copy_field_clicked),
                              value_copy, (GClosureNotify)g_free, 0);
        adw_action_row_add_suffix(ADW_ACTION_ROW(row), copy_btn);

        adw_preferences_group_add(ADW_PREFERENCES_GROUP(group), row);
    }
    g_list_free(members);

    gtk_box_append(GTK_BOX(vd->detail_fields_box), group);

    gboolean has_totp = json_object_get_boolean_member_with_default(obj, "has_totp", FALSE);
    const char *secret = vault_get_totp_secret(obj);
    if (has_totp && secret && *secret) {
        GtkWidget *totp_group = adw_preferences_group_new();
        adw_preferences_group_set_title(ADW_PREFERENCES_GROUP(totp_group), "One-Time Password");
        gtk_widget_set_margin_top(totp_group, 12);
        gtk_widget_set_margin_start(totp_group, 4);
        gtk_widget_set_margin_end(totp_group, 4);

        GtkWidget *totp_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 4);
        gtk_widget_set_margin_start(totp_box, 12);
        gtk_widget_set_margin_end(totp_box, 12);
        gtk_widget_set_margin_top(totp_box, 8);
        gtk_widget_set_margin_bottom(totp_box, 8);

        GtkWidget *totp_code_row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
        gtk_widget_set_valign(totp_code_row, GTK_ALIGN_CENTER);

        vd->detail_totp_code = gtk_label_new("--- ---");
        gtk_widget_add_css_class(vd->detail_totp_code, "title-1");
        gtk_label_set_xalign(GTK_LABEL(vd->detail_totp_code), 0);
        gtk_widget_set_hexpand(vd->detail_totp_code, TRUE);
        gtk_box_append(GTK_BOX(totp_code_row), vd->detail_totp_code);

        GtkWidget *totp_copy_btn = gtk_button_new_from_icon_name("edit-copy-symbolic");
        gtk_widget_add_css_class(totp_copy_btn, "flat");
        gtk_widget_set_valign(totp_copy_btn, GTK_ALIGN_CENTER);
        g_signal_connect(totp_copy_btn, "clicked",
                         G_CALLBACK(on_copy_totp_clicked), vd->detail_totp_code);
        gtk_box_append(GTK_BOX(totp_code_row), totp_copy_btn);

        gtk_box_append(GTK_BOX(totp_box), totp_code_row);

        vd->detail_totp_bar = gtk_level_bar_new_for_interval(0.0, 1.0);
        gtk_level_bar_set_value(GTK_LEVEL_BAR(vd->detail_totp_bar), 1.0);
        gtk_widget_add_css_class(vd->detail_totp_bar, "totp-progress");
        gtk_widget_set_margin_top(vd->detail_totp_bar, 4);
        gtk_box_append(GTK_BOX(totp_box), vd->detail_totp_bar);

        adw_preferences_group_add(ADW_PREFERENCES_GROUP(totp_group), totp_box);
        gtk_box_append(GTK_BOX(vd->detail_fields_box), totp_group);

        vd->detail_totp_secret = g_strdup(secret);
        vault_update_totp(vd);
        vd->detail_totp_timer = g_timeout_add_seconds(1, vault_update_totp, vd);
    }

    gtk_widget_set_visible(vd->detail_action_bar, TRUE);
    gtk_stack_set_visible_child_name(GTK_STACK(vd->detail_stack), "detail");
}

/* delete */

static void on_delete_response(AdwAlertDialog *dialog, const char *response, gpointer data)
{
    (void)dialog;
    VaultData *vd = data;
    if (g_strcmp0(response, "delete") != 0) return;
    if (!vd->selected_id) return;

    SolockClient *client = solock_app_get_client(vd->app);
    GError *error = NULL;
    if (!solock_client_delete_entry(client, vd->selected_id, &error)) {
        g_warning("Delete failed: %s", error->message);
        g_error_free(error);
        return;
    }

    vault_clear_detail(vd);
    vault_refresh_entries(vd);
}

static void on_delete_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    VaultData *vd = data;
    if (!vd->selected_id) return;

    AdwAlertDialog *dialog = ADW_ALERT_DIALOG(adw_alert_dialog_new(
        "Delete Entry?", "This action cannot be undone."));
    adw_alert_dialog_add_responses(dialog, "cancel", "Cancel", "delete", "Delete", NULL);
    adw_alert_dialog_set_response_appearance(dialog, "delete", ADW_RESPONSE_DESTRUCTIVE);
    adw_alert_dialog_set_default_response(dialog, "cancel");
    g_signal_connect(dialog, "response", G_CALLBACK(on_delete_response), vd);

    GtkWidget *toplevel = gtk_widget_get_ancestor(vd->list_box, GTK_TYPE_WINDOW);
    adw_dialog_present(ADW_DIALOG(dialog), toplevel);
}

/* edit */

static void on_edit_save_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    VaultData *vd = data;
    if (!vd->selected_id) return;

    const char *new_name = gtk_editable_get_text(GTK_EDITABLE(vd->edit_name_entry));
    if (!new_name || !*new_name) return;

    JsonBuilder *builder = json_builder_new();
    json_builder_begin_object(builder);

    for (GtkWidget *child = gtk_widget_get_first_child(vd->edit_fields_box);
         child != NULL;
         child = gtk_widget_get_next_sibling(child)) {

        const char *key = g_object_get_data(G_OBJECT(child), "field-key");
        GtkWidget *entry = g_object_get_data(G_OBJECT(child), "field-entry");
        if (!key || !entry) continue;

        const char *val = gtk_editable_get_text(GTK_EDITABLE(entry));
        json_builder_set_member_name(builder, key);
        json_builder_add_string_value(builder, val ? val : "");
    }

    json_builder_end_object(builder);
    JsonNode *fields_node = json_builder_get_root(builder);
    g_object_unref(builder);

    SolockClient *client = solock_app_get_client(vd->app);
    GError *error = NULL;
    if (!solock_client_update_entry(client, vd->selected_id, new_name, fields_node, &error)) {
        g_warning("Update failed: %s", error->message);
        g_error_free(error);
        json_node_unref(fields_node);
        return;
    }
    json_node_unref(fields_node);

    vd->editing = FALSE;
    vault_refresh_entries(vd);

    /* reload detail */
    GError *err2 = NULL;
    JsonNode *updated = solock_client_get_entry(client, vd->selected_id, &err2);
    if (updated) {
        vault_show_detail(vd, json_node_get_object(updated));
        json_node_unref(updated);
    } else {
        if (err2) g_error_free(err2);
        vault_clear_detail(vd);
    }
}

static void on_edit_cancel_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    VaultData *vd = data;
    if (!vd->selected_id) return;

    vd->editing = FALSE;

    SolockClient *client = solock_app_get_client(vd->app);
    GError *error = NULL;
    JsonNode *entry = solock_client_get_entry(client, vd->selected_id, &error);
    if (entry) {
        vault_show_detail(vd, json_node_get_object(entry));
        json_node_unref(entry);
    } else {
        if (error) g_error_free(error);
        vault_clear_detail(vd);
    }
}

static void on_generate_password(GtkButton *button, gpointer data)
{
    (void)button;
    VaultData *vd = data;
    SolockClient *client = solock_app_get_client(vd->app);
    GError *error = NULL;
    char *password = solock_client_generate_password(client, 24, TRUE, TRUE, TRUE, &error);
    if (!password) {
        if (error) {
            g_warning("Password generation failed: %s", error->message);
            g_error_free(error);
        }
        return;
    }

    GtkWidget *entry = g_object_get_data(G_OBJECT(button), "password-entry");
    if (entry)
        gtk_editable_set_text(GTK_EDITABLE(entry), password);
    g_free(password);
}

static GtkWidget *make_edit_field_row(VaultData *vd, const char *key, const char *value)
{
    GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_top(row, 4);
    gtk_widget_set_margin_bottom(row, 4);
    g_object_set_data(G_OBJECT(row), "field-key", (gpointer)key);

    GtkWidget *label = gtk_label_new(human_label(key));
    gtk_widget_add_css_class(label, "dim-label");
    gtk_widget_set_size_request(label, 100, -1);
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_box_append(GTK_BOX(row), label);

    GtkWidget *entry = gtk_entry_new();
    gtk_editable_set_text(GTK_EDITABLE(entry), value ? value : "");
    gtk_widget_set_hexpand(entry, TRUE);
    if (is_sensitive_field(key))
        gtk_entry_set_visibility(GTK_ENTRY(entry), FALSE);
    gtk_box_append(GTK_BOX(row), entry);
    g_object_set_data(G_OBJECT(row), "field-entry", entry);

    if (g_strcmp0(key, "password") == 0) {
        GtkWidget *gen_btn = gtk_button_new_from_icon_name("system-run-symbolic");
        gtk_widget_add_css_class(gen_btn, "flat");
        gtk_widget_set_tooltip_text(gen_btn, "Generate password");
        g_object_set_data(G_OBJECT(gen_btn), "password-entry", entry);
        g_signal_connect(gen_btn, "clicked", G_CALLBACK(on_generate_password), vd);
        gtk_box_append(GTK_BOX(row), gen_btn);
    }

    return row;
}

static void on_edit_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    VaultData *vd = data;
    if (!vd->selected_id || !vd->entries) return;

    JsonArray *arr = json_node_get_array(vd->entries);
    guint len = json_array_get_length(arr);
    JsonObject *obj = NULL;

    for (guint i = 0; i < len; i++) {
        JsonObject *cur = json_array_get_object_element(arr, i);
        const char *cur_id = json_object_get_string_member(cur, "id");
        if (g_strcmp0(cur_id, vd->selected_id) == 0) {
            obj = cur;
            break;
        }
    }
    if (!obj) return;

    vd->editing = TRUE;
    gtk_widget_set_visible(vd->detail_action_bar, FALSE);

    const char *name = json_object_get_string_member(obj, "name");
    JsonObject *fields = json_object_get_object_member(obj, "fields");

    vault_clear_container(vd->detail_fields_box);

    /* name edit */
    GtkWidget *name_row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_top(name_row, 4);
    gtk_widget_set_margin_bottom(name_row, 8);
    GtkWidget *name_lbl = gtk_label_new("Name");
    gtk_widget_add_css_class(name_lbl, "dim-label");
    gtk_widget_set_size_request(name_lbl, 100, -1);
    gtk_label_set_xalign(GTK_LABEL(name_lbl), 0);
    gtk_box_append(GTK_BOX(name_row), name_lbl);

    vd->edit_name_entry = gtk_entry_new();
    gtk_editable_set_text(GTK_EDITABLE(vd->edit_name_entry), name);
    gtk_widget_set_hexpand(vd->edit_name_entry, TRUE);
    gtk_box_append(GTK_BOX(name_row), vd->edit_name_entry);

    gtk_box_append(GTK_BOX(vd->detail_fields_box), name_row);

    /* edit fields box */
    vd->edit_fields_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);

    if (fields) {
        GList *members = json_object_get_members(fields);
        for (GList *l = members; l; l = l->next) {
            const char *key = l->data;
            const char *value = json_object_get_string_member(fields, key);
            gtk_box_append(GTK_BOX(vd->edit_fields_box),
                           make_edit_field_row(vd, key, value));
        }
        g_list_free(members);
    }
    gtk_box_append(GTK_BOX(vd->detail_fields_box), vd->edit_fields_box);

    /* buttons */
    GtkWidget *btn_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_top(btn_box, 12);
    gtk_widget_set_halign(btn_box, GTK_ALIGN_END);

    vd->edit_cancel_btn = gtk_button_new_with_label("Cancel");
    g_signal_connect(vd->edit_cancel_btn, "clicked", G_CALLBACK(on_edit_cancel_clicked), vd);
    gtk_box_append(GTK_BOX(btn_box), vd->edit_cancel_btn);

    vd->edit_save_btn = gtk_button_new_with_label("Save");
    gtk_widget_add_css_class(vd->edit_save_btn, "suggested-action");
    g_signal_connect(vd->edit_save_btn, "clicked", G_CALLBACK(on_edit_save_clicked), vd);
    gtk_box_append(GTK_BOX(btn_box), vd->edit_save_btn);

    gtk_box_append(GTK_BOX(vd->detail_fields_box), btn_box);
}

/* add entry */

static void on_add_type_changed(GObject *object, GParamSpec *pspec, gpointer data)
{
    (void)object; (void)pspec;
    VaultData *vd = data;
    vault_clear_container(vd->add_fields_box);

    guint idx = gtk_drop_down_get_selected(GTK_DROP_DOWN(vd->add_type_dropdown));
    if (idx >= (guint)entry_types_count) return;

    const EntryTypeDef *def = &entry_types[idx];
    for (const char **f = def->fields; *f; f++) {
        gtk_box_append(GTK_BOX(vd->add_fields_box),
                       make_edit_field_row(vd, *f, ""));
    }
}

static void on_add_save_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    VaultData *vd = data;

    const char *name = gtk_editable_get_text(GTK_EDITABLE(vd->add_name_entry));

    guint type_idx = gtk_drop_down_get_selected(GTK_DROP_DOWN(vd->add_type_dropdown));
    if (type_idx >= (guint)entry_types_count) return;

    const char *type = entry_types[type_idx].type;

    const char *site_value = NULL;
    JsonBuilder *builder = json_builder_new();
    json_builder_begin_object(builder);

    for (GtkWidget *child = gtk_widget_get_first_child(vd->add_fields_box);
         child != NULL;
         child = gtk_widget_get_next_sibling(child)) {

        const char *key = g_object_get_data(G_OBJECT(child), "field-key");
        GtkWidget *entry = g_object_get_data(G_OBJECT(child), "field-entry");
        if (!key || !entry) continue;

        const char *val = gtk_editable_get_text(GTK_EDITABLE(entry));
        json_builder_set_member_name(builder, key);
        json_builder_add_string_value(builder, val ? val : "");

        if (g_strcmp0(key, "site") == 0 && val && *val)
            site_value = val;
    }

    json_builder_end_object(builder);

    if (!name || !*name) {
        if (site_value)
            name = site_value;
        else {
            json_node_unref(json_builder_get_root(builder));
            g_object_unref(builder);
            return;
        }
    }
    JsonNode *fields_node = json_builder_get_root(builder);
    g_object_unref(builder);

    SolockClient *client = solock_app_get_client(vd->app);
    GError *error = NULL;
    if (!solock_client_add_entry(client, type, name, fields_node, &error)) {
        g_warning("Add entry failed: %s", error->message);
        g_error_free(error);
        json_node_unref(fields_node);
        return;
    }
    json_node_unref(fields_node);

    /* reset form */
    gtk_editable_set_text(GTK_EDITABLE(vd->add_name_entry), "");
    vault_clear_container(vd->add_fields_box);
    gtk_drop_down_set_selected(GTK_DROP_DOWN(vd->add_type_dropdown), 0);
    on_add_type_changed(NULL, NULL, vd);

    gtk_stack_set_visible_child_name(GTK_STACK(vd->add_stack), "list");
    vault_refresh_entries(vd);
    vault_clear_detail(vd);
}

static void on_add_cancel_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    VaultData *vd = data;
    gtk_stack_set_visible_child_name(GTK_STACK(vd->add_stack), "list");
}

static void on_add_button_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    VaultData *vd = data;
    gtk_editable_set_text(GTK_EDITABLE(vd->add_name_entry), "");
    gtk_drop_down_set_selected(GTK_DROP_DOWN(vd->add_type_dropdown), 0);
    on_add_type_changed(NULL, NULL, vd);
    gtk_stack_set_visible_child_name(GTK_STACK(vd->add_stack), "add");
}

/* build */

static GtkWidget *build_add_form(VaultData *vd)
{
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 8);
    gtk_widget_set_margin_start(box, 12);
    gtk_widget_set_margin_end(box, 12);
    gtk_widget_set_margin_top(box, 12);
    gtk_widget_set_margin_bottom(box, 12);

    GtkWidget *title = gtk_label_new("New Entry");
    gtk_widget_add_css_class(title, "title-3");
    gtk_label_set_xalign(GTK_LABEL(title), 0);
    gtk_box_append(GTK_BOX(box), title);

    /* type dropdown */
    GtkWidget *type_row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_top(type_row, 8);
    GtkWidget *type_lbl = gtk_label_new("Type");
    gtk_widget_add_css_class(type_lbl, "dim-label");
    gtk_widget_set_size_request(type_lbl, 100, -1);
    gtk_label_set_xalign(GTK_LABEL(type_lbl), 0);
    gtk_box_append(GTK_BOX(type_row), type_lbl);

    const char *type_options[] = { "Password", "Note", "Card", "TOTP", NULL };
    vd->add_type_dropdown = gtk_drop_down_new_from_strings(type_options);
    gtk_widget_set_hexpand(vd->add_type_dropdown, TRUE);
    g_signal_connect(vd->add_type_dropdown, "notify::selected",
                     G_CALLBACK(on_add_type_changed), vd);
    gtk_box_append(GTK_BOX(type_row), vd->add_type_dropdown);
    gtk_box_append(GTK_BOX(box), type_row);

    /* name */
    GtkWidget *name_row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_top(name_row, 4);
    GtkWidget *name_lbl = gtk_label_new("Name");
    gtk_widget_add_css_class(name_lbl, "dim-label");
    gtk_widget_set_size_request(name_lbl, 100, -1);
    gtk_label_set_xalign(GTK_LABEL(name_lbl), 0);
    gtk_box_append(GTK_BOX(name_row), name_lbl);

    vd->add_name_entry = gtk_entry_new();
    gtk_widget_set_hexpand(vd->add_name_entry, TRUE);
    gtk_entry_set_placeholder_text(GTK_ENTRY(vd->add_name_entry), "Entry name");
    gtk_box_append(GTK_BOX(name_row), vd->add_name_entry);
    gtk_box_append(GTK_BOX(box), name_row);

    /* dynamic fields */
    vd->add_fields_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_box_append(GTK_BOX(box), vd->add_fields_box);

    /* buttons */
    GtkWidget *btn_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_top(btn_box, 12);
    gtk_widget_set_halign(btn_box, GTK_ALIGN_END);

    GtkWidget *cancel_btn = gtk_button_new_with_label("Cancel");
    g_signal_connect(cancel_btn, "clicked", G_CALLBACK(on_add_cancel_clicked), vd);
    gtk_box_append(GTK_BOX(btn_box), cancel_btn);

    vd->add_save_btn = gtk_button_new_with_label("Save");
    gtk_widget_add_css_class(vd->add_save_btn, "suggested-action");
    g_signal_connect(vd->add_save_btn, "clicked", G_CALLBACK(on_add_save_clicked), vd);
    gtk_box_append(GTK_BOX(btn_box), vd->add_save_btn);

    gtk_box_append(GTK_BOX(box), btn_box);

    return box;
}

GtkWidget *solock_vault_view_new(SolockApp *app)
{
    VaultData *vd = g_new0(VaultData, 1);
    vd->app = app;

    GtkWidget *paned = gtk_paned_new(GTK_ORIENTATION_HORIZONTAL);
    gtk_paned_set_position(GTK_PANED(paned), 320);
    gtk_paned_set_shrink_start_child(GTK_PANED(paned), FALSE);
    gtk_paned_set_shrink_end_child(GTK_PANED(paned), FALSE);

    /* left: list + search + add form */
    vd->add_stack = gtk_stack_new();
    gtk_stack_set_transition_type(GTK_STACK(vd->add_stack), GTK_STACK_TRANSITION_TYPE_CROSSFADE);

    GtkWidget *list_panel = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);

    /* toolbar with search + add */
    GtkWidget *toolbar = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    gtk_widget_set_margin_start(toolbar, 8);
    gtk_widget_set_margin_end(toolbar, 8);
    gtk_widget_set_margin_top(toolbar, 8);
    gtk_widget_set_margin_bottom(toolbar, 4);

    vd->search_entry = gtk_search_entry_new();
    gtk_widget_set_hexpand(vd->search_entry, TRUE);
    g_signal_connect(vd->search_entry, "search-changed", G_CALLBACK(on_search_changed), vd);
    gtk_box_append(GTK_BOX(toolbar), vd->search_entry);

    GtkWidget *add_btn = gtk_button_new_from_icon_name("list-add-symbolic");
    gtk_widget_add_css_class(add_btn, "flat");
    gtk_widget_set_tooltip_text(add_btn, "Add entry");
    g_signal_connect(add_btn, "clicked", G_CALLBACK(on_add_button_clicked), vd);
    gtk_box_append(GTK_BOX(toolbar), add_btn);

    gtk_box_append(GTK_BOX(list_panel), toolbar);

    /* scrollable list */
    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll),
                                   GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
    gtk_widget_set_vexpand(scroll, TRUE);

    vd->list_box = gtk_list_box_new();
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(vd->list_box), GTK_SELECTION_SINGLE);
    gtk_widget_add_css_class(vd->list_box, "navigation-sidebar");
    g_signal_connect(vd->list_box, "row-activated", G_CALLBACK(on_entry_row_activated), vd);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), vd->list_box);
    gtk_box_append(GTK_BOX(list_panel), scroll);

    gtk_stack_add_named(GTK_STACK(vd->add_stack), list_panel, "list");
    gtk_stack_add_named(GTK_STACK(vd->add_stack), build_add_form(vd), "add");
    gtk_stack_set_visible_child_name(GTK_STACK(vd->add_stack), "list");

    gtk_paned_set_start_child(GTK_PANED(paned), vd->add_stack);

    /* right: detail panel */
    vd->detail_stack = gtk_stack_new();
    gtk_stack_set_transition_type(GTK_STACK(vd->detail_stack), GTK_STACK_TRANSITION_TYPE_CROSSFADE);

    /* empty state - summary */
    vd->detail_empty = gtk_box_new(GTK_ORIENTATION_VERTICAL, 12);
    gtk_widget_set_valign(vd->detail_empty, GTK_ALIGN_CENTER);
    gtk_widget_set_halign(vd->detail_empty, GTK_ALIGN_CENTER);
    gtk_widget_set_margin_start(vd->detail_empty, 24);
    gtk_widget_set_margin_end(vd->detail_empty, 24);

    GtkWidget *empty_icon = gtk_image_new_from_icon_name("dialog-password-symbolic");
    gtk_image_set_pixel_size(GTK_IMAGE(empty_icon), 48);
    gtk_widget_add_css_class(empty_icon, "dim-label");
    gtk_box_append(GTK_BOX(vd->detail_empty), empty_icon);

    GtkWidget *stats_group = adw_preferences_group_new();
    adw_preferences_group_set_title(ADW_PREFERENCES_GROUP(stats_group), "Summary");

    vd->stats_total = adw_action_row_new();
    adw_preferences_row_set_title(ADW_PREFERENCES_ROW(vd->stats_total), "Total Entries");
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_total), "0");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(stats_group), vd->stats_total);

    vd->stats_passwords = adw_action_row_new();
    adw_preferences_row_set_title(ADW_PREFERENCES_ROW(vd->stats_passwords), "Passwords");
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_passwords), "0");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(stats_group), vd->stats_passwords);

    vd->stats_cards = adw_action_row_new();
    adw_preferences_row_set_title(ADW_PREFERENCES_ROW(vd->stats_cards), "Cards");
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_cards), "0");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(stats_group), vd->stats_cards);

    vd->stats_notes = adw_action_row_new();
    adw_preferences_row_set_title(ADW_PREFERENCES_ROW(vd->stats_notes), "Notes");
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_notes), "0");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(stats_group), vd->stats_notes);

    vd->stats_totp = adw_action_row_new();
    adw_preferences_row_set_title(ADW_PREFERENCES_ROW(vd->stats_totp), "With 2FA");
    adw_action_row_set_subtitle(ADW_ACTION_ROW(vd->stats_totp), "0");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(stats_group), vd->stats_totp);

    gtk_box_append(GTK_BOX(vd->detail_empty), stats_group);
    gtk_stack_add_named(GTK_STACK(vd->detail_stack), vd->detail_empty, "empty");

    /* detail view */
    GtkWidget *detail_scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(detail_scroll),
                                   GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);

    vd->detail_view = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_widget_set_valign(vd->detail_view, GTK_ALIGN_START);
    gtk_widget_set_margin_start(vd->detail_view, 16);
    gtk_widget_set_margin_end(vd->detail_view, 16);
    gtk_widget_set_margin_top(vd->detail_view, 12);
    gtk_widget_set_margin_bottom(vd->detail_view, 12);

    /* header */
    GtkWidget *detail_header = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_bottom(detail_header, 12);

    vd->detail_name_label = gtk_label_new("");
    gtk_widget_add_css_class(vd->detail_name_label, "title-2");
    gtk_label_set_xalign(GTK_LABEL(vd->detail_name_label), 0);
    gtk_label_set_ellipsize(GTK_LABEL(vd->detail_name_label), PANGO_ELLIPSIZE_END);
    gtk_widget_set_hexpand(vd->detail_name_label, TRUE);
    gtk_box_append(GTK_BOX(detail_header), vd->detail_name_label);

    vd->detail_type_label = gtk_label_new("");
    gtk_widget_add_css_class(vd->detail_type_label, "dim-label");
    gtk_widget_set_valign(vd->detail_type_label, GTK_ALIGN_CENTER);
    gtk_box_append(GTK_BOX(detail_header), vd->detail_type_label);

    gtk_box_append(GTK_BOX(vd->detail_view), detail_header);

    /* fields container */
    vd->detail_fields_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_box_append(GTK_BOX(vd->detail_view), vd->detail_fields_box);

    /* action buttons */
    vd->detail_action_bar = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_top(vd->detail_action_bar, 16);
    gtk_widget_set_halign(vd->detail_action_bar, GTK_ALIGN_END);

    vd->detail_edit_btn = gtk_button_new_with_label("Edit");
    gtk_widget_add_css_class(vd->detail_edit_btn, "flat");
    g_signal_connect(vd->detail_edit_btn, "clicked", G_CALLBACK(on_edit_clicked), vd);
    gtk_box_append(GTK_BOX(vd->detail_action_bar), vd->detail_edit_btn);

    vd->detail_delete_btn = gtk_button_new_with_label("Delete");
    gtk_widget_add_css_class(vd->detail_delete_btn, "destructive-action");
    g_signal_connect(vd->detail_delete_btn, "clicked", G_CALLBACK(on_delete_clicked), vd);
    gtk_box_append(GTK_BOX(vd->detail_action_bar), vd->detail_delete_btn);

    gtk_box_append(GTK_BOX(vd->detail_view), vd->detail_action_bar);

    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(detail_scroll), vd->detail_view);
    gtk_stack_add_named(GTK_STACK(vd->detail_stack), detail_scroll, "detail");

    gtk_stack_set_visible_child_name(GTK_STACK(vd->detail_stack), "empty");

    gtk_paned_set_end_child(GTK_PANED(paned), vd->detail_stack);

    vault_refresh_entries(vd);
    gtk_list_box_unselect_all(GTK_LIST_BOX(vd->list_box));

    return paned;
}

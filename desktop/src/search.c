#include "solock-desktop.h"
#include <string.h>

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);

static const char *LABEL_CHARS = "asdfghjkl";

typedef struct {
    SolockApp  *app;
    GtkWidget  *search_entry;
    GtkWidget  *list_box;
    GtkWidget  *fields_stack;
    JsonNode   *entries;
    gboolean    label_mode;
} SearchData;

static void refresh_entries(SearchData *sd, const char *query);

static char *label_for_index(int idx)
{
    int base = strlen(LABEL_CHARS);
    if (idx < base) {
        return g_strdup_printf("%c", LABEL_CHARS[idx]);
    }
    int first = idx / base;
    int second = idx % base;
    if (first >= base) first = base - 1;
    return g_strdup_printf("%c%c", LABEL_CHARS[first], LABEL_CHARS[second]);
}

static void on_entry_activated(GtkListBox *list_box, GtkListBoxRow *row, gpointer data)
{
    (void)list_box;
    SearchData *sd = data;
    int idx = gtk_list_box_row_get_index(row);

    JsonArray *arr = json_node_get_array(sd->entries);
    if (idx < 0 || (guint)idx >= json_array_get_length(arr)) return;

    JsonNode *entry = json_array_get_element(arr, idx);

    GtkWidget *fields = solock_fields_view_new(sd->app, entry);
    gtk_stack_add_named(GTK_STACK(sd->fields_stack), fields, "fields");
    gtk_stack_set_visible_child_name(GTK_STACK(sd->fields_stack), "fields");
}

static void on_search_changed(GtkEditable *editable, gpointer data)
{
    SearchData *sd = data;
    const char *text = gtk_editable_get_text(editable);
    refresh_entries(sd, text);
}

static void refresh_entries(SearchData *sd, const char *query)
{
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(sd->list_box)) != NULL)
        gtk_list_box_remove(GTK_LIST_BOX(sd->list_box), child);

    if (sd->entries) json_node_unref(sd->entries);

    SolockClient *client = solock_app_get_client(sd->app);
    GError *error = NULL;

    if (query && *query)
        sd->entries = solock_client_search_entries(client, query, &error);
    else
        sd->entries = solock_client_list_entries(client, &error);

    if (!sd->entries) return;

    JsonArray *arr = json_node_get_array(sd->entries);
    guint len = json_array_get_length(arr);

    for (guint i = 0; i < len; i++) {
        JsonObject *obj = json_array_get_object_element(arr, i);
        const char *name = json_object_get_string_member(obj, "name");
        const char *type = json_object_get_string_member(obj, "type");

        GtkWidget *row_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
        gtk_widget_set_margin_start(row_box, 8);
        gtk_widget_set_margin_end(row_box, 8);
        gtk_widget_set_margin_top(row_box, 4);
        gtk_widget_set_margin_bottom(row_box, 4);

        if (sd->label_mode) {
            char *lbl = label_for_index(i);
            GtkWidget *label_w = gtk_label_new(lbl);
            gtk_widget_add_css_class(label_w, "label-hint");
            gtk_box_append(GTK_BOX(row_box), label_w);
            g_free(lbl);
        }

        GtkWidget *name_label = gtk_label_new(name);
        gtk_widget_set_hexpand(name_label, TRUE);
        gtk_label_set_xalign(GTK_LABEL(name_label), 0);
        gtk_box_append(GTK_BOX(row_box), name_label);

        GtkWidget *type_label = gtk_label_new(type);
        gtk_widget_add_css_class(type_label, "dim-label");
        gtk_widget_add_css_class(type_label, "caption");
        gtk_box_append(GTK_BOX(row_box), type_label);

        if (json_object_get_boolean_member_with_default(obj, "has_totp", FALSE)) {
            GtkWidget *totp_badge = gtk_label_new("2FA");
            gtk_widget_add_css_class(totp_badge, "accent");
            gtk_widget_add_css_class(totp_badge, "caption");
            gtk_box_append(GTK_BOX(row_box), totp_badge);
        }

        gtk_list_box_append(GTK_LIST_BOX(sd->list_box), row_box);
    }
}

GtkWidget *solock_search_view_new(SolockApp *app)
{
    GtkWidget *stack = gtk_stack_new();
    gtk_stack_set_transition_type(GTK_STACK(stack), GTK_STACK_TRANSITION_TYPE_SLIDE_LEFT_RIGHT);

    GtkWidget *main_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);

    GtkWidget *search_entry = gtk_search_entry_new();
    gtk_widget_set_margin_start(search_entry, 12);
    gtk_widget_set_margin_end(search_entry, 12);
    gtk_widget_set_margin_top(search_entry, 12);
    gtk_box_append(GTK_BOX(main_box), search_entry);

    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
    gtk_widget_set_vexpand(scroll, TRUE);

    GtkWidget *list_box = gtk_list_box_new();
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(list_box), GTK_SELECTION_SINGLE);
    gtk_widget_add_css_class(list_box, "boxed-list");
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), list_box);
    gtk_box_append(GTK_BOX(main_box), scroll);

    gtk_stack_add_named(GTK_STACK(stack), main_box, "list");

    SearchData *sd = g_new0(SearchData, 1);
    sd->app = app;
    sd->search_entry = search_entry;
    sd->list_box = list_box;
    sd->fields_stack = stack;
    sd->entries = NULL;
    sd->label_mode = FALSE;

    g_signal_connect(search_entry, "search-changed", G_CALLBACK(on_search_changed), sd);
    g_signal_connect(list_box, "row-activated", G_CALLBACK(on_entry_activated), sd);

    refresh_entries(sd, "");

    return stack;
}

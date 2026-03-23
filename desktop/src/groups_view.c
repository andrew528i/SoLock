#include "solock-desktop.h"
#include <string.h>

extern SolockClient *solock_app_get_client(SolockApp *app);

typedef struct {
    SolockApp     *app;
    GtkListBox    *list_box;
    GtkWidget     *empty_label;
    GtkWidget     *spinner;
    GtkStack      *view_stack;
    GtkEntry      *name_entry;
    int            edit_index;
} GroupsData;

static void refresh_groups(GroupsData *gd);
static void groups_show_spinner(GroupsData *gd, gboolean show);

typedef struct {
    GroupsData *gd;
    int         index;
    gboolean    deleted;
    char       *name;
} RowActionData;

static void row_action_data_free(gpointer data, GClosure *closure)
{
    (void)closure;
    RowActionData *ra = data;
    g_free(ra->name);
    g_free(ra);
}

static void on_row_edit(GtkButton *btn, gpointer data)
{
    (void)btn;
    RowActionData *ra = data;
    ra->gd->edit_index = ra->index;
    gtk_editable_set_text(GTK_EDITABLE(ra->gd->name_entry), ra->name ? ra->name : "");
    gtk_stack_set_visible_child_name(ra->gd->view_stack, "edit");
    gtk_widget_grab_focus(GTK_WIDGET(ra->gd->name_entry));
}

static void on_row_delete(GtkButton *btn, gpointer data)
{
    (void)btn;
    RowActionData *ra = data;
    SolockClient *client = solock_app_get_client(ra->gd->app);
    GError *error = NULL;

    groups_show_spinner(ra->gd, TRUE);

    if (ra->deleted) {
        solock_client_purge_group(client, ra->index, &error);
    } else {
        solock_client_delete_group(client, ra->index, FALSE, &error);
    }

    groups_show_spinner(ra->gd, FALSE);

    if (error) {
        g_warning("Group action error: %s", error->message);
        g_error_free(error);
    }

    refresh_groups(ra->gd);
}

static void on_add_clicked(GtkButton *btn, gpointer data)
{
    (void)btn;
    GroupsData *gd = data;
    gd->edit_index = -1;
    gtk_editable_set_text(GTK_EDITABLE(gd->name_entry), "");
    gtk_stack_set_visible_child_name(gd->view_stack, "edit");
    gtk_widget_grab_focus(GTK_WIDGET(gd->name_entry));
}

static void groups_show_spinner(GroupsData *gd, gboolean show)
{
    if (gd->spinner) {
        gtk_widget_set_visible(gd->spinner, show);
        if (show) gtk_spinner_start(GTK_SPINNER(gd->spinner));
        else gtk_spinner_stop(GTK_SPINNER(gd->spinner));
    }
}

static void on_save_clicked(GtkButton *btn, gpointer data)
{
    (void)btn;
    GroupsData *gd = data;
    SolockClient *client = solock_app_get_client(gd->app);
    const char *name = gtk_editable_get_text(GTK_EDITABLE(gd->name_entry));

    if (!name || strlen(name) == 0) return;

    groups_show_spinner(gd, TRUE);

    GError *error = NULL;
    if (gd->edit_index < 0) {
        solock_client_add_group(client, name, &error);
    } else {
        solock_client_update_group(client, gd->edit_index, name, &error);
    }

    groups_show_spinner(gd, FALSE);

    if (error) {
        g_warning("Group save error: %s", error->message);
        g_error_free(error);
    }

    gtk_stack_set_visible_child_name(gd->view_stack, "list");
    refresh_groups(gd);
}

static void on_cancel_clicked(GtkButton *btn, gpointer data)
{
    (void)btn;
    GroupsData *gd = data;
    gtk_stack_set_visible_child_name(gd->view_stack, "list");
}

static void refresh_groups(GroupsData *gd)
{
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(GTK_WIDGET(gd->list_box))))
        gtk_list_box_remove(gd->list_box, child);

    SolockClient *client = solock_app_get_client(gd->app);
    if (solock_client_is_locked(client)) {
        gtk_widget_set_visible(gd->empty_label, TRUE);
        return;
    }

    GError *error = NULL;
    JsonNode *result = solock_client_list_groups(client, &error);

    if (error || !result) {
        gtk_widget_set_visible(gd->empty_label, TRUE);
        if (error) g_error_free(error);
        return;
    }

    JsonArray *arr = json_node_get_array(result);
    guint len = json_array_get_length(arr);

    gtk_widget_set_visible(gd->empty_label, len == 0);

    for (guint i = 0; i < len; i++) {
        JsonObject *obj = json_array_get_object_element(arr, i);
        const char *name = json_object_get_string_member(obj, "name");
        int index = (int)json_object_get_int_member(obj, "index");
        gboolean deleted = json_object_get_boolean_member(obj, "deleted");

        GtkWidget *row_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
        gtk_widget_set_margin_start(row_box, 8);
        gtk_widget_set_margin_end(row_box, 8);
        gtk_widget_set_margin_top(row_box, 6);
        gtk_widget_set_margin_bottom(row_box, 6);

        GtkWidget *icon = gtk_image_new_from_icon_name(
            deleted ? "user-trash-symbolic" : "folder-symbolic");
        gtk_image_set_pixel_size(GTK_IMAGE(icon), 16);
        gtk_box_append(GTK_BOX(row_box), icon);

        GtkWidget *label = gtk_label_new(name);
        gtk_label_set_xalign(GTK_LABEL(label), 0);
        gtk_widget_set_hexpand(label, TRUE);
        if (deleted)
            gtk_widget_add_css_class(label, "dim-label");
        gtk_box_append(GTK_BOX(row_box), label);

        if (deleted) {
            GtkWidget *del_badge = gtk_label_new("deleted");
            gtk_widget_add_css_class(del_badge, "dim-label");
            gtk_box_append(GTK_BOX(row_box), del_badge);
        }

        if (!deleted) {
            RowActionData *edit_data = g_new0(RowActionData, 1);
            edit_data->gd = gd;
            edit_data->index = index;
            edit_data->name = g_strdup(name);
            GtkWidget *edit_btn = gtk_button_new_from_icon_name("document-edit-symbolic");
            gtk_widget_add_css_class(edit_btn, "flat");
            gtk_widget_set_tooltip_text(edit_btn, "Rename");
            g_signal_connect_data(edit_btn, "clicked", G_CALLBACK(on_row_edit),
                                 edit_data, (GClosureNotify)row_action_data_free, 0);
            gtk_box_append(GTK_BOX(row_box), edit_btn);
        }

        RowActionData *del_data = g_new0(RowActionData, 1);
        del_data->gd = gd;
        del_data->index = index;
        del_data->deleted = deleted;
        del_data->name = g_strdup(name);
        GtkWidget *del_btn = gtk_button_new_from_icon_name(
            deleted ? "edit-clear-all-symbolic" : "user-trash-symbolic");
        gtk_widget_add_css_class(del_btn, "flat");
        gtk_widget_set_tooltip_text(del_btn, deleted ? "Purge permanently" : "Delete");
        if (!deleted)
            gtk_widget_add_css_class(del_btn, "error");
        g_signal_connect_data(del_btn, "clicked", G_CALLBACK(on_row_delete),
                             del_data, (GClosureNotify)row_action_data_free, 0);
        gtk_box_append(GTK_BOX(row_box), del_btn);

        gtk_list_box_append(gd->list_box, row_box);
    }

    json_node_unref(result);
}

GtkWidget *solock_groups_view_new(SolockApp *app)
{
    GroupsData *gd = g_new0(GroupsData, 1);
    gd->app = app;
    gd->edit_index = -1;

    GtkWidget *stack = gtk_stack_new();
    gd->view_stack = GTK_STACK(stack);

    /* list page */
    GtkWidget *list_page = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);

    GtkWidget *toolbar = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    gtk_widget_set_margin_start(toolbar, 8);
    gtk_widget_set_margin_end(toolbar, 8);
    gtk_widget_set_margin_top(toolbar, 8);
    gtk_widget_set_margin_bottom(toolbar, 4);

    GtkWidget *title_label = gtk_label_new("Groups");
    gtk_widget_add_css_class(title_label, "title-4");
    gtk_widget_set_hexpand(title_label, TRUE);
    gtk_label_set_xalign(GTK_LABEL(title_label), 0);
    gtk_widget_set_margin_start(title_label, 4);
    gtk_box_append(GTK_BOX(toolbar), title_label);

    gd->spinner = gtk_spinner_new();
    gtk_widget_set_visible(gd->spinner, FALSE);
    gtk_box_append(GTK_BOX(toolbar), gd->spinner);

    GtkWidget *add_btn = gtk_button_new_from_icon_name("list-add-symbolic");
    gtk_widget_add_css_class(add_btn, "flat");
    gtk_widget_set_tooltip_text(add_btn, "Add group");
    g_signal_connect(add_btn, "clicked", G_CALLBACK(on_add_clicked), gd);
    gtk_box_append(GTK_BOX(toolbar), add_btn);

    gtk_box_append(GTK_BOX(list_page), toolbar);

    GtkWidget *sep = gtk_separator_new(GTK_ORIENTATION_HORIZONTAL);
    gtk_box_append(GTK_BOX(list_page), sep);

    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_widget_set_vexpand(scroll, TRUE);

    GtkWidget *list_box = gtk_list_box_new();
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(list_box), GTK_SELECTION_NONE);
    gtk_widget_set_margin_start(list_box, 8);
    gtk_widget_set_margin_end(list_box, 8);
    gtk_widget_set_margin_top(list_box, 4);
    gd->list_box = GTK_LIST_BOX(list_box);

    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), list_box);
    gtk_box_append(GTK_BOX(list_page), scroll);

    GtkWidget *empty_label = gtk_label_new("No groups yet. Click + to create one.");
    gtk_widget_add_css_class(empty_label, "dim-label");
    gtk_widget_set_margin_top(empty_label, 32);
    gtk_widget_set_margin_bottom(empty_label, 32);
    gd->empty_label = empty_label;
    gtk_box_append(GTK_BOX(list_page), empty_label);

    gtk_stack_add_named(GTK_STACK(stack), list_page, "list");

    /* edit page */
    GtkWidget *edit_page = gtk_box_new(GTK_ORIENTATION_VERTICAL, 12);
    gtk_widget_set_margin_start(edit_page, 24);
    gtk_widget_set_margin_end(edit_page, 24);
    gtk_widget_set_margin_top(edit_page, 24);

    GtkWidget *name_label = gtk_label_new("Group Name");
    gtk_label_set_xalign(GTK_LABEL(name_label), 0);
    gtk_box_append(GTK_BOX(edit_page), name_label);

    GtkWidget *name_entry = gtk_entry_new();
    gtk_entry_set_max_length(GTK_ENTRY(name_entry), 64);
    gtk_entry_set_placeholder_text(GTK_ENTRY(name_entry), "e.g. Work, Personal");
    gd->name_entry = GTK_ENTRY(name_entry);
    gtk_box_append(GTK_BOX(edit_page), name_entry);

    GtkWidget *btn_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_top(btn_box, 12);

    GtkWidget *save_btn = gtk_button_new_with_label("Save");
    gtk_widget_add_css_class(save_btn, "suggested-action");
    g_signal_connect(save_btn, "clicked", G_CALLBACK(on_save_clicked), gd);
    gtk_box_append(GTK_BOX(btn_box), save_btn);

    GtkWidget *cancel_btn = gtk_button_new_with_label("Cancel");
    g_signal_connect(cancel_btn, "clicked", G_CALLBACK(on_cancel_clicked), gd);
    gtk_box_append(GTK_BOX(btn_box), cancel_btn);

    gtk_box_append(GTK_BOX(edit_page), btn_box);
    gtk_stack_add_named(GTK_STACK(stack), edit_page, "edit");

    gtk_stack_set_visible_child_name(GTK_STACK(stack), "list");

    return stack;
}

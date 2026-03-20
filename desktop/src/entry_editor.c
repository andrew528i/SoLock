#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);

GtkWidget *solock_entry_editor_new(SolockApp *app, JsonNode *entry)
{
    (void)app;
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 12);
    gtk_widget_set_margin_start(box, 16);
    gtk_widget_set_margin_end(box, 16);
    gtk_widget_set_margin_top(box, 16);
    gtk_widget_set_margin_bottom(box, 16);

    const char *title_text = "New Entry";
    if (entry) {
        JsonObject *obj = json_node_get_object(entry);
        title_text = json_object_get_string_member(obj, "name");
    }

    GtkWidget *title = gtk_label_new(title_text);
    gtk_widget_add_css_class(title, "title-3");
    gtk_label_set_xalign(GTK_LABEL(title), 0);
    gtk_box_append(GTK_BOX(box), title);

    GtkWidget *placeholder = gtk_label_new("Entry editor - coming soon");
    gtk_widget_add_css_class(placeholder, "dim-label");
    gtk_box_append(GTK_BOX(box), placeholder);

    return box;
}

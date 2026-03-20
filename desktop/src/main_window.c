#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);

static void on_sidebar_row_selected(GtkListBox *list, GtkListBoxRow *row, gpointer data)
{
    (void)list;
    GtkStack *stack = data;
    if (!row) return;
    int idx = gtk_list_box_row_get_index(row);
    const char *names[] = { "dashboard", "vault", "settings" };
    if (idx >= 0 && idx < 3)
        gtk_stack_set_visible_child_name(stack, names[idx]);
}

GtkWidget *solock_main_window_new(SolockApp *app)
{
    GtkWidget *win = adw_application_window_new(GTK_APPLICATION(app));
    gtk_window_set_title(GTK_WINDOW(win), "SoLock");
    gtk_window_set_default_size(GTK_WINDOW(win), 900, 600);

    GtkWidget *split = adw_navigation_split_view_new();

    /* sidebar */
    GtkWidget *sidebar_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    GtkWidget *sidebar_header = adw_header_bar_new();
    adw_header_bar_set_title_widget(ADW_HEADER_BAR(sidebar_header),
                                     adw_window_title_new("SoLock", ""));
    gtk_box_append(GTK_BOX(sidebar_box), sidebar_header);

    GtkWidget *sidebar_list = gtk_list_box_new();
    gtk_widget_add_css_class(sidebar_list, "navigation-sidebar");
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(sidebar_list), GTK_SELECTION_SINGLE);

    const char *sections[] = { "Dashboard", "Vault", "Settings" };
    for (int i = 0; i < 3; i++) {
        GtkWidget *row = gtk_label_new(sections[i]);
        gtk_label_set_xalign(GTK_LABEL(row), 0);
        gtk_widget_set_margin_start(row, 12);
        gtk_widget_set_margin_end(row, 12);
        gtk_widget_set_margin_top(row, 8);
        gtk_widget_set_margin_bottom(row, 8);
        gtk_list_box_append(GTK_LIST_BOX(sidebar_list), row);
    }
    gtk_box_append(GTK_BOX(sidebar_box), sidebar_list);

    GtkWidget *sidebar_page = adw_navigation_page_new(sidebar_box, "Navigation");
    adw_navigation_split_view_set_sidebar(ADW_NAVIGATION_SPLIT_VIEW(split), sidebar_page);

    /* content stack */
    GtkWidget *content_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    GtkWidget *content_header = adw_header_bar_new();
    gtk_box_append(GTK_BOX(content_box), content_header);

    GtkWidget *content_stack = gtk_stack_new();
    gtk_widget_set_vexpand(content_stack, TRUE);

    GtkWidget *dashboard = solock_dashboard_view_new(app);
    gtk_stack_add_named(GTK_STACK(content_stack), dashboard, "dashboard");

    GtkWidget *vault_scroll = gtk_scrolled_window_new();
    gtk_stack_add_named(GTK_STACK(content_stack), vault_scroll, "vault");

    GtkWidget *settings = solock_settings_view_new(app);
    gtk_stack_add_named(GTK_STACK(content_stack), settings, "settings");

    gtk_box_append(GTK_BOX(content_box), content_stack);

    GtkWidget *content_page = adw_navigation_page_new(content_box, "Content");
    adw_navigation_split_view_set_content(ADW_NAVIGATION_SPLIT_VIEW(split), content_page);

    g_signal_connect(sidebar_list, "row-selected",
                     G_CALLBACK(on_sidebar_row_selected), content_stack);

    adw_application_window_set_content(ADW_APPLICATION_WINDOW(win), split);

    return win;
}

#include "solock-desktop.h"

typedef struct {
    GtkStack  *stack;
    GtkWidget *content_title;
    GtkWindow *window;
} MainWindowData;

static const char *section_names[] = { "vault", "dashboard", "settings" };
static const char *section_titles[] = { "Vault", "Dashboard", "Settings" };
static const char *sidebar_icon_names[] = {
    "dialog-password-symbolic",
    "utilities-system-monitor-symbolic",
    "preferences-system-symbolic"
};

static void on_sidebar_row_selected(GtkListBox *list, GtkListBoxRow *row, gpointer data)
{
    (void)list;
    MainWindowData *mwd = data;
    if (!row) return;
    int idx = gtk_list_box_row_get_index(row);
    if (idx >= 0 && idx < 3) {
        gtk_stack_set_visible_child_name(mwd->stack, section_names[idx]);
        adw_window_title_set_title(ADW_WINDOW_TITLE(mwd->content_title),
                                   section_titles[idx]);
    }
}

GtkWidget *solock_main_window_new(SolockApp *app)
{
    GtkWidget *win = adw_application_window_new(GTK_APPLICATION(app));
    gtk_window_set_title(GTK_WINDOW(win), "SoLock");
    gtk_window_set_default_size(GTK_WINDOW(win), 850, 550);
    gtk_window_set_deletable(GTK_WINDOW(win), FALSE);

    AdwNavigationSplitView *split = ADW_NAVIGATION_SPLIT_VIEW(adw_navigation_split_view_new());

    /* sidebar */
    GtkWidget *sidebar_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);

    GtkWidget *sidebar_header = adw_header_bar_new();
    adw_header_bar_set_title_widget(ADW_HEADER_BAR(sidebar_header),
                                     GTK_WIDGET(adw_window_title_new("SoLock", "")));
    adw_header_bar_set_show_end_title_buttons(ADW_HEADER_BAR(sidebar_header), FALSE);
    adw_header_bar_set_show_start_title_buttons(ADW_HEADER_BAR(sidebar_header), FALSE);
    gtk_box_append(GTK_BOX(sidebar_box), sidebar_header);

    GtkWidget *sidebar_list = gtk_list_box_new();
    gtk_widget_add_css_class(sidebar_list, "navigation-sidebar");
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(sidebar_list), GTK_SELECTION_SINGLE);
    gtk_widget_set_vexpand(sidebar_list, TRUE);

    for (int i = 0; i < 3; i++) {
        GtkWidget *row_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
        gtk_widget_set_margin_start(row_box, 14);
        gtk_widget_set_margin_end(row_box, 14);
        gtk_widget_set_margin_top(row_box, 12);
        gtk_widget_set_margin_bottom(row_box, 12);

        GtkWidget *img = gtk_image_new_from_icon_name(sidebar_icon_names[i]);
        gtk_image_set_pixel_size(GTK_IMAGE(img), 16);
        gtk_box_append(GTK_BOX(row_box), img);

        GtkWidget *label = gtk_label_new(section_titles[i]);
        gtk_label_set_xalign(GTK_LABEL(label), 0);
        gtk_box_append(GTK_BOX(row_box), label);

        gtk_list_box_append(GTK_LIST_BOX(sidebar_list), row_box);
    }

    gtk_box_append(GTK_BOX(sidebar_box), sidebar_list);

    AdwNavigationPage *sidebar_page = adw_navigation_page_new(sidebar_box, "Navigation");
    adw_navigation_split_view_set_sidebar(split, sidebar_page);

    /* content */
    GtkWidget *content_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    GtkWidget *content_header = adw_header_bar_new();
    GtkWidget *content_title = GTK_WIDGET(adw_window_title_new(section_titles[0], ""));
    adw_header_bar_set_title_widget(ADW_HEADER_BAR(content_header), content_title);
    adw_header_bar_set_show_start_title_buttons(ADW_HEADER_BAR(content_header), FALSE);
    adw_header_bar_set_show_end_title_buttons(ADW_HEADER_BAR(content_header), FALSE);
    gtk_box_append(GTK_BOX(content_box), content_header);

    GtkWidget *content_stack = gtk_stack_new();
    gtk_widget_set_vexpand(content_stack, TRUE);

    GtkWidget *vault = solock_vault_view_new(app);
    gtk_stack_add_named(GTK_STACK(content_stack), vault, "vault");

    GtkWidget *dashboard = solock_dashboard_view_new(app);
    gtk_stack_add_named(GTK_STACK(content_stack), dashboard, "dashboard");

    GtkWidget *settings = solock_settings_view_new(app);
    gtk_stack_add_named(GTK_STACK(content_stack), settings, "settings");

    gtk_stack_set_visible_child_name(GTK_STACK(content_stack), "vault");
    gtk_box_append(GTK_BOX(content_box), content_stack);

    AdwNavigationPage *content_page = adw_navigation_page_new(content_box, "Content");
    adw_navigation_split_view_set_content(split, content_page);

    MainWindowData *mwd = g_new0(MainWindowData, 1);
    mwd->stack = GTK_STACK(content_stack);
    mwd->content_title = content_title;
    mwd->window = GTK_WINDOW(win);

    g_signal_connect(sidebar_list, "row-selected",
                     G_CALLBACK(on_sidebar_row_selected), mwd);

    GtkListBoxRow *first = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sidebar_list), 0);
    if (first)
        gtk_list_box_select_row(GTK_LIST_BOX(sidebar_list), first);

    adw_application_window_set_content(ADW_APPLICATION_WINDOW(win), GTK_WIDGET(split));

    return win;
}

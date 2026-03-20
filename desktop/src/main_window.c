#include "solock-desktop.h"

typedef struct {
    GtkStack  *stack;
    GtkWidget *content_title;
    GtkWidget *sidebar_list;
} MainWindowData;

static const char *section_names[] = { "vault", "dashboard", "settings" };
static const char *section_titles[] = { "Vault", "Dashboard", "Settings" };

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

static void on_quit_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    GtkWindow *win = data;
    gtk_window_close(win);
}

GtkWidget *solock_main_window_new(SolockApp *app)
{
    GtkWidget *win = adw_application_window_new(GTK_APPLICATION(app));
    gtk_window_set_title(GTK_WINDOW(win), "SoLock");
    gtk_window_set_default_size(GTK_WINDOW(win), 850, 550);

    AdwNavigationSplitView *split = ADW_NAVIGATION_SPLIT_VIEW(adw_navigation_split_view_new());

    /* sidebar */
    GtkWidget *sidebar_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    GtkWidget *sidebar_header = adw_header_bar_new();
    adw_header_bar_set_title_widget(ADW_HEADER_BAR(sidebar_header),
                                     GTK_WIDGET(adw_window_title_new("SoLock", "")));
    gtk_box_append(GTK_BOX(sidebar_box), sidebar_header);

    GtkWidget *sidebar_list = gtk_list_box_new();
    gtk_widget_add_css_class(sidebar_list, "navigation-sidebar");
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(sidebar_list), GTK_SELECTION_SINGLE);
    gtk_widget_set_vexpand(sidebar_list, FALSE);

    for (int i = 0; i < 3; i++) {
        GtkWidget *label = gtk_label_new(section_titles[i]);
        gtk_label_set_xalign(GTK_LABEL(label), 0);
        gtk_widget_set_margin_start(label, 12);
        gtk_widget_set_margin_end(label, 12);
        gtk_widget_set_margin_top(label, 8);
        gtk_widget_set_margin_bottom(label, 8);
        gtk_list_box_append(GTK_LIST_BOX(sidebar_list), label);
    }
    gtk_box_append(GTK_BOX(sidebar_box), sidebar_list);

    GtkWidget *spacer = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_widget_set_vexpand(spacer, TRUE);
    gtk_box_append(GTK_BOX(sidebar_box), spacer);

    GtkWidget *quit_btn = gtk_button_new_with_label("Quit");
    gtk_widget_add_css_class(quit_btn, "flat");
    gtk_widget_set_margin_start(quit_btn, 12);
    gtk_widget_set_margin_end(quit_btn, 12);
    gtk_widget_set_margin_bottom(quit_btn, 12);
    gtk_widget_set_valign(quit_btn, GTK_ALIGN_END);
    g_signal_connect(quit_btn, "clicked", G_CALLBACK(on_quit_clicked), win);
    gtk_box_append(GTK_BOX(sidebar_box), quit_btn);

    AdwNavigationPage *sidebar_page = adw_navigation_page_new(sidebar_box, "Navigation");
    adw_navigation_split_view_set_sidebar(split, sidebar_page);

    /* content */
    GtkWidget *content_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    GtkWidget *content_header = adw_header_bar_new();
    GtkWidget *content_title = GTK_WIDGET(adw_window_title_new(section_titles[0], ""));
    adw_header_bar_set_title_widget(ADW_HEADER_BAR(content_header), content_title);
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
    mwd->sidebar_list = sidebar_list;

    g_signal_connect(sidebar_list, "row-selected",
                     G_CALLBACK(on_sidebar_row_selected), mwd);

    GtkListBoxRow *first = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sidebar_list), 0);
    if (first)
        gtk_list_box_select_row(GTK_LIST_BOX(sidebar_list), first);

    adw_application_window_set_content(ADW_APPLICATION_WINDOW(win), GTK_WIDGET(split));

    return win;
}

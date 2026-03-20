#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);

typedef struct {
    SolockApp *app;
    GtkWidget *entry;
    GtkWidget *error_label;
    GtkWidget *spinner;
} UnlockData;

static void on_unlock_activate(GtkEntry *widget, gpointer data)
{
    (void)widget;
    UnlockData *ud = data;
    const char *password = gtk_editable_get_text(GTK_EDITABLE(ud->entry));
    if (!password || !*password) return;

    gtk_widget_set_visible(ud->spinner, TRUE);
    gtk_spinner_start(GTK_SPINNER(ud->spinner));
    gtk_label_set_text(GTK_LABEL(ud->error_label), "");

    SolockClient *client = solock_app_get_client(ud->app);
    SolockConfig *config = solock_app_get_config(ud->app);
    int timeout = solock_config_get_timeout_minutes(config);

    GError *error = NULL;
    gboolean ok = solock_client_unlock(client, password, timeout, &error);

    gtk_spinner_stop(GTK_SPINNER(ud->spinner));
    gtk_widget_set_visible(ud->spinner, FALSE);
    gtk_editable_set_text(GTK_EDITABLE(ud->entry), "");

    if (ok) {
        solock_tray_update_status(ud->app, FALSE);
        solock_client_sync(client, NULL);
        GtkWidget *popup = solock_app_get_popup(ud->app);
        solock_popup_hide(popup);
    } else {
        gtk_label_set_text(GTK_LABEL(ud->error_label), error->message);
        g_error_free(error);
    }
}

static void on_view_map(GtkWidget *widget, gpointer data)
{
    (void)widget;
    UnlockData *ud = data;
    gtk_widget_grab_focus(ud->entry);
}

GtkWidget *solock_unlock_view_new(SolockApp *app)
{
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 8);
    gtk_widget_set_halign(box, GTK_ALIGN_CENTER);
    gtk_widget_set_valign(box, GTK_ALIGN_CENTER);
    gtk_widget_set_margin_start(box, 40);
    gtk_widget_set_margin_end(box, 40);
    gtk_widget_set_margin_top(box, 20);
    gtk_widget_set_margin_bottom(box, 20);

    GtkWidget *icon = gtk_image_new_from_icon_name("system-lock-screen-symbolic");
    gtk_image_set_pixel_size(GTK_IMAGE(icon), 28);
    gtk_widget_add_css_class(icon, "unlock-icon");
    gtk_box_append(GTK_BOX(box), icon);

    GtkWidget *title = gtk_label_new("SoLock");
    gtk_widget_add_css_class(title, "unlock-title");
    gtk_box_append(GTK_BOX(box), title);

    GtkWidget *entry = gtk_password_entry_new();
    gtk_password_entry_set_show_peek_icon(GTK_PASSWORD_ENTRY(entry), TRUE);
    gtk_widget_set_size_request(entry, 220, -1);
    gtk_box_append(GTK_BOX(box), entry);

    GtkWidget *spinner = gtk_spinner_new();
    gtk_widget_set_visible(spinner, FALSE);
    gtk_box_append(GTK_BOX(box), spinner);

    GtkWidget *error_label = gtk_label_new("");
    gtk_widget_add_css_class(error_label, "error");
    gtk_box_append(GTK_BOX(box), error_label);

    UnlockData *ud = g_new0(UnlockData, 1);
    ud->app = app;
    ud->entry = entry;
    ud->error_label = error_label;
    ud->spinner = spinner;

    g_signal_connect(entry, "activate", G_CALLBACK(on_unlock_activate), ud);
    g_signal_connect(box, "map", G_CALLBACK(on_view_map), ud);

    return box;
}

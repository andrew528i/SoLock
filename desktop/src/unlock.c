#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);

typedef struct {
    SolockApp *app;
    GtkWidget *entry;
    GtkWidget *status_label;
    GtkWidget *spinner;
} UnlockData;

static void on_unlock_activate(GtkEntry *entry, gpointer data)
{
    UnlockData *ud = data;
    const char *password = gtk_editable_get_text(GTK_EDITABLE(ud->entry));
    if (!password || !*password) return;

    gtk_widget_set_visible(ud->spinner, TRUE);
    gtk_label_set_text(GTK_LABEL(ud->status_label), "");

    SolockClient *client = solock_app_get_client(ud->app);
    SolockConfig *config = solock_app_get_config(ud->app);
    int timeout = solock_config_get_timeout_minutes(config);

    GError *error = NULL;
    gboolean ok = solock_client_unlock(client, password, timeout, &error);

    gtk_widget_set_visible(ud->spinner, FALSE);
    gtk_editable_set_text(GTK_EDITABLE(ud->entry), "");

    if (ok) {
        solock_tray_update_status(ud->app, FALSE);
        GtkWidget *popup = solock_app_get_popup(ud->app);
        solock_popup_show(popup);
    } else {
        gtk_label_set_text(GTK_LABEL(ud->status_label), error->message);
        g_error_free(error);
    }
}

GtkWidget *solock_unlock_view_new(SolockApp *app)
{
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 12);
    gtk_widget_set_halign(box, GTK_ALIGN_CENTER);
    gtk_widget_set_valign(box, GTK_ALIGN_CENTER);
    gtk_widget_set_margin_start(box, 24);
    gtk_widget_set_margin_end(box, 24);
    gtk_widget_set_margin_top(box, 24);
    gtk_widget_set_margin_bottom(box, 24);

    GtkWidget *title = gtk_label_new("Master Password");
    gtk_widget_add_css_class(title, "title-3");
    gtk_box_append(GTK_BOX(box), title);

    GtkWidget *entry = gtk_password_entry_new();
    gtk_password_entry_set_show_peek_icon(GTK_PASSWORD_ENTRY(entry), TRUE);
    gtk_widget_set_size_request(entry, 300, -1);
    gtk_box_append(GTK_BOX(box), entry);

    GtkWidget *spinner = gtk_spinner_new();
    gtk_widget_set_visible(spinner, FALSE);
    gtk_box_append(GTK_BOX(box), spinner);

    GtkWidget *status = gtk_label_new("");
    gtk_widget_add_css_class(status, "error");
    gtk_box_append(GTK_BOX(box), status);

    UnlockData *ud = g_new0(UnlockData, 1);
    ud->app = app;
    ud->entry = entry;
    ud->status_label = status;
    ud->spinner = spinner;

    g_signal_connect(entry, "activate", G_CALLBACK(on_unlock_activate), ud);

    return box;
}

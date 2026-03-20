#include "solock-desktop.h"

struct _SolockApp {
    AdwApplication  parent;
    SolockClient   *client;
    SolockConfig   *config;
    GtkWidget      *popup;
    GtkWidget      *main_window;
    guint           expiry_timer;
};

typedef struct _SolockAppClass {
    AdwApplicationClass parent_class;
} SolockAppClass;

G_DEFINE_TYPE(SolockApp, solock_app, ADW_TYPE_APPLICATION)

static void on_activate(GApplication *gapp)
{
    SolockApp *app = (SolockApp *)gapp;

    AdwStyleManager *style = adw_style_manager_get_default();
    adw_style_manager_set_color_scheme(style, ADW_COLOR_SCHEME_PREFER_DARK);

    if (app->popup) {
        solock_popup_show(app->popup);
        return;
    }

    if (!solock_wtype_available()) {
        g_message("wtype not found, auto-paste will use clipboard fallback");
    }

    GError *error = NULL;
    if (!solock_client_start_serve(app->client, &error)) {
        g_warning("Failed to start solock serve: %s", error->message);
        g_error_free(error);
        return;
    }

    solock_tray_setup(app);

    app->popup = solock_popup_new(app);
    solock_popup_show(app->popup);
}

static gboolean on_expiry_check(gpointer data)
{
    SolockApp *app = data;
    if (!solock_client_is_locked(app->client)) {
        JsonNode *status = solock_client_status(app->client, NULL);
        if (status) {
            JsonObject *obj = json_node_get_object(status);
            if (json_object_has_member(obj, "remaining_seconds")) {
                gint64 remaining = json_object_get_int_member(obj, "remaining_seconds");
                if (remaining <= 0) {
                    solock_client_lock(app->client);
                    solock_tray_update_status(app, TRUE);
                }
            }
            json_node_unref(status);
        }
    }
    return G_SOURCE_CONTINUE;
}

static void solock_app_startup(GApplication *gapp)
{
    G_APPLICATION_CLASS(solock_app_parent_class)->startup(gapp);
    SolockApp *app = (SolockApp *)gapp;

    app->config = solock_config_new();
    solock_config_load(app->config);

    app->expiry_timer = g_timeout_add_seconds(30, on_expiry_check, app);
}

static void solock_app_shutdown_handler(GApplication *gapp)
{
    SolockApp *app = (SolockApp *)gapp;

    if (app->expiry_timer > 0) {
        g_source_remove(app->expiry_timer);
        app->expiry_timer = 0;
    }

    solock_client_shutdown(app->client);
    solock_client_stop_serve(app->client);
    solock_client_free(app->client);
    solock_config_free(app->config);

    G_APPLICATION_CLASS(solock_app_parent_class)->shutdown(gapp);
}

static void solock_app_init(SolockApp *app)
{
    app->client = solock_client_new();
    app->config = NULL;
    app->popup = NULL;
    app->main_window = NULL;
    app->expiry_timer = 0;
}

static void solock_app_class_init(SolockAppClass *klass)
{
    GApplicationClass *app_class = G_APPLICATION_CLASS(klass);
    app_class->activate = on_activate;
    app_class->startup = solock_app_startup;
    app_class->shutdown = solock_app_shutdown_handler;
}

SolockApp *solock_app_new(void)
{
    return g_object_new(solock_app_get_type(),
                        "application-id", SOLOCK_APP_ID,
                        "flags", G_APPLICATION_DEFAULT_FLAGS,
                        NULL);
}

SolockClient *solock_app_get_client(SolockApp *app) { return app->client; }
SolockConfig *solock_app_get_config(SolockApp *app) { return app->config; }
GtkWidget    *solock_app_get_popup(SolockApp *app)  { return app->popup; }

void solock_app_show_main_window(SolockApp *app)
{
    if (app->main_window == NULL) {
        app->main_window = solock_main_window_new(app);
    }
    gtk_window_present(GTK_WINDOW(app->main_window));
}

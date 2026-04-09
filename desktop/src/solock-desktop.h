#pragma once

#include <adwaita.h>
#include <gtk4-layer-shell.h>
#include <json-glib/json-glib.h>

#define SOLOCK_APP_ID "com.solock.desktop"
#define SOLOCK_BINARY "solock"

typedef struct _SolockApp SolockApp;
typedef struct _SolockClient SolockClient;
typedef struct _SolockConfig SolockConfig;

/* client.h */
SolockClient *solock_client_new(void);
void          solock_client_free(SolockClient *client);
gboolean      solock_client_start_serve(SolockClient *client, GError **error);
void          solock_client_stop_serve(SolockClient *client);
gboolean      solock_client_is_connected(SolockClient *client);
JsonNode     *solock_client_call(SolockClient *client, const char *method, JsonNode *params, GError **error);
gboolean      solock_client_unlock(SolockClient *client, const char *password, int timeout_minutes, GError **error);
void          solock_client_lock(SolockClient *client);
gboolean      solock_client_is_locked(SolockClient *client);
JsonNode     *solock_client_list_entries(SolockClient *client, GError **error);
JsonNode     *solock_client_search_entries(SolockClient *client, const char *query, GError **error);
JsonNode     *solock_client_get_entry(SolockClient *client, const char *id, GError **error);
JsonNode     *solock_client_get_dashboard(SolockClient *client, GError **error);
gboolean      solock_client_add_entry(SolockClient *client, const char *type, const char *name, JsonNode *fields, int group_index, GError **error);
gboolean      solock_client_update_entry(SolockClient *client, const char *id, const char *name, JsonNode *fields, int group_index, gboolean clear_group, GError **error);
gboolean      solock_client_delete_entry(SolockClient *client, const char *id, GError **error);
gboolean      solock_client_sync(SolockClient *client, GError **error);
JsonNode     *solock_client_list_groups(SolockClient *client, GError **error);
gboolean      solock_client_add_group(SolockClient *client, const char *name, const char *color, GError **error);
gboolean      solock_client_update_group(SolockClient *client, int index, const char *name, const char *color, GError **error);
gboolean      solock_client_delete_group(SolockClient *client, int index, gboolean delete_entries, GError **error);
gboolean      solock_client_purge_group(SolockClient *client, int index, GError **error);
gboolean      solock_client_deploy_program(SolockClient *client, GError **error);
gboolean      solock_client_init_vault(SolockClient *client, GError **error);
char         *solock_client_generate_password(SolockClient *client, int length, gboolean uppercase, gboolean digits, gboolean special, GError **error);
JsonNode     *solock_client_generate_totp(SolockClient *client, const char *secret, int digits, int period, GError **error);
JsonNode     *solock_client_status(SolockClient *client, GError **error);
JsonNode     *solock_client_sync_status(SolockClient *client, GError **error);
void          solock_client_shutdown(SolockClient *client);

/* config.h */
SolockConfig *solock_config_new(void);
void          solock_config_free(SolockConfig *config);
gboolean      solock_config_load(SolockConfig *config);
gboolean      solock_config_save(SolockConfig *config);
int           solock_config_get_timeout_minutes(SolockConfig *config);
void          solock_config_set_timeout_minutes(SolockConfig *config, int minutes);
int           solock_config_get_clipboard_clear_seconds(SolockConfig *config);
void          solock_config_set_clipboard_clear_seconds(SolockConfig *config, int seconds);
const char   *solock_config_get_paste_method(SolockConfig *config);
void          solock_config_set_paste_method(SolockConfig *config, const char *method);

/* application.h */
SolockApp    *solock_app_new(void);

/* popup.h */
GtkWidget    *solock_popup_new(SolockApp *app);
void          solock_popup_show(GtkWidget *popup);
void          solock_popup_hide(GtkWidget *popup);
void          solock_popup_toggle(GtkWidget *popup);
void          solock_popup_switch_to_search(GtkWidget *popup);
void          solock_popup_switch_to_detail(GtkWidget *popup, JsonNode *entry);

/* unlock.h */
GtkWidget    *solock_unlock_view_new(SolockApp *app);

/* search.h */
GtkWidget    *solock_search_view_new(SolockApp *app);

/* fields.h */
GtkWidget    *solock_fields_view_new(SolockApp *app, JsonNode *entry);
void          solock_fields_reset_label_mode(void);
void          solock_fields_set_label_mode(gboolean mode);

/* main_window.h */
GtkWidget    *solock_main_window_new(SolockApp *app);

/* dashboard.h */
GtkWidget    *solock_dashboard_view_new(SolockApp *app);

/* vault_view.h */
GtkWidget    *solock_vault_view_new(SolockApp *app);

/* groups_view.h */
GtkWidget    *solock_groups_view_new(SolockApp *app);

/* entry_editor.h */
GtkWidget    *solock_entry_editor_new(SolockApp *app, JsonNode *entry);

/* settings.h */
GtkWidget    *solock_settings_view_new(SolockApp *app);

/* tray.h */
void          solock_tray_setup(SolockApp *app);
void          solock_tray_update_status(SolockApp *app, gboolean locked);
void          solock_tray_set_syncing(SolockApp *app, gboolean syncing);

/* wtype.h */
gboolean      solock_wtype_available(void);
gboolean      solock_wtype_type(const char *text, GError **error);

/* clipboard.h */
gboolean      solock_clipboard_copy(const char *text, int clear_after_seconds, GError **error);

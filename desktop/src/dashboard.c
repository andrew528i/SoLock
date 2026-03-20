#include "solock-desktop.h"
#include <string.h>

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);

typedef struct {
    SolockApp *app;
    GtkWidget *deployer_row;
    GtkWidget *program_row;
    GtkWidget *balance_row;
    GtkWidget *network_label;
    GtkWidget *program_status_row;
    GtkWidget *vault_status_row;
    GtkWidget *entries_row;
    GtkWidget *passwords_row;
    GtkWidget *notes_row;
    GtkWidget *cards_row;
    GtkWidget *totps_row;
    GtkWidget *sync_row;
    GtkWidget *rent_row;
    GtkWidget *refresh_btn;
    GtkWidget *spinner;
    char *deployer_text;
    char *program_text;
} DashboardData;

static void on_copy_deployer(GtkButton *button, gpointer data)
{
    (void)button;
    DashboardData *dd = data;
    if (dd->deployer_text)
        solock_clipboard_copy(dd->deployer_text, 0, NULL);
}

static void on_copy_program(GtkButton *button, gpointer data)
{
    (void)button;
    DashboardData *dd = data;
    if (dd->program_text)
        solock_clipboard_copy(dd->program_text, 0, NULL);
}

static void set_row_value(GtkWidget *row, const char *value)
{
    adw_action_row_set_subtitle(ADW_ACTION_ROW(row), value);
}

static void dashboard_refresh_done(DashboardData *dd, JsonNode *info)
{
    gtk_spinner_stop(GTK_SPINNER(dd->spinner));
    gtk_widget_set_visible(dd->spinner, FALSE);
    gtk_widget_set_sensitive(dd->refresh_btn, TRUE);

    if (!info) return;

    JsonObject *obj = json_node_get_object(info);

    const char *deployer = json_object_get_string_member(obj, "deployer_address");
    set_row_value(dd->deployer_row, deployer ? deployer : "-");
    g_free(dd->deployer_text);
    dd->deployer_text = g_strdup(deployer);

    const char *program_id = json_object_get_string_member(obj, "program_id");
    set_row_value(dd->program_row, program_id ? program_id : "-");
    g_free(dd->program_text);
    dd->program_text = g_strdup(program_id);

    gint64 balance = json_object_get_int_member(obj, "balance");
    char *bal_str = g_strdup_printf("%.4f SOL", (double)balance / 1e9);
    set_row_value(dd->balance_row, bal_str);
    g_free(bal_str);

    const char *network = json_object_get_string_member(obj, "network");
    if (!network || !*network)
        network = "devnet";
    gtk_label_set_text(GTK_LABEL(dd->network_label), network);
    gtk_widget_remove_css_class(dd->network_label, "success");
    gtk_widget_remove_css_class(dd->network_label, "warning");
    if (g_strcmp0(network, "devnet") == 0)
        gtk_widget_add_css_class(dd->network_label, "success");
    else
        gtk_widget_add_css_class(dd->network_label, "warning");

    gboolean deployed = json_object_get_boolean_member(obj, "program_deployed");
    set_row_value(dd->program_status_row, deployed ? "Deployed" : "Not deployed");

    gboolean vault = json_object_get_boolean_member(obj, "vault_exists");
    set_row_value(dd->vault_status_row, vault ? "Ready" : "Not initialized");

    gint64 entry_count = json_object_get_int_member(obj, "entry_count");
    char *entries_str = g_strdup_printf("%lld", (long long)entry_count);
    set_row_value(dd->entries_row, entries_str);
    g_free(entries_str);

    double rent = (double)entry_count * 0.002 + 0.007;
    char *rent_str = g_strdup_printf("%.3f SOL", rent);
    set_row_value(dd->rent_row, rent_str);
    g_free(rent_str);

    gint64 pw_count = json_object_get_int_member(obj, "password_count");
    char *pw_str = g_strdup_printf("%lld", (long long)pw_count);
    set_row_value(dd->passwords_row, pw_str);
    g_free(pw_str);

    gint64 note_count = json_object_get_int_member(obj, "note_count");
    char *note_str = g_strdup_printf("%lld", (long long)note_count);
    set_row_value(dd->notes_row, note_str);
    g_free(note_str);

    gint64 card_count = json_object_get_int_member(obj, "card_count");
    char *card_str = g_strdup_printf("%lld", (long long)card_count);
    set_row_value(dd->cards_row, card_str);
    g_free(card_str);

    gint64 totp_count = json_object_get_int_member(obj, "totp_count");
    char *totp_str = g_strdup_printf("%lld", (long long)totp_count);
    set_row_value(dd->totps_row, totp_str);
    g_free(totp_str);

    gint64 last_sync = json_object_get_int_member(obj, "last_sync_at");
    if (last_sync > 0) {
        GDateTime *dt = g_date_time_new_from_unix_local(last_sync);
        if (dt) {
            char *sync_str = g_date_time_format(dt, "%Y-%m-%d %H:%M:%S");
            set_row_value(dd->sync_row, sync_str);
            g_free(sync_str);
            g_date_time_unref(dt);
        }
    } else {
        set_row_value(dd->sync_row, "Never");
    }

    json_node_unref(info);
}

static void dashboard_refresh(DashboardData *dd)
{
    SolockClient *client = solock_app_get_client(dd->app);
    if (solock_client_is_locked(client)) return;

    gtk_widget_set_sensitive(dd->refresh_btn, FALSE);
    gtk_widget_set_visible(dd->spinner, TRUE);
    gtk_spinner_start(GTK_SPINNER(dd->spinner));

    GError *error = NULL;
    JsonNode *info = solock_client_get_dashboard(client, &error);
    if (error) {
        g_warning("Dashboard refresh failed: %s", error->message);
        g_error_free(error);
    }
    dashboard_refresh_done(dd, info);
}

static void on_refresh_clicked(GtkButton *button, gpointer data)
{
    (void)button;
    dashboard_refresh(data);
}

static void on_clear_local_db_clicked(GtkButton *button, gpointer data)
{
    (void)button; (void)data;
    g_message("Clear Local DB: not yet implemented");
}

static GtkWidget *make_action_row(const char *title, const char *subtitle)
{
    GtkWidget *row = adw_action_row_new();
    adw_preferences_row_set_title(ADW_PREFERENCES_ROW(row), title);
    adw_action_row_set_subtitle(ADW_ACTION_ROW(row), subtitle);
    adw_action_row_set_subtitle_selectable(ADW_ACTION_ROW(row), TRUE);
    gtk_widget_set_margin_top(row, 4);
    gtk_widget_set_margin_bottom(row, 4);
    gtk_widget_set_margin_start(row, 8);
    gtk_widget_set_margin_end(row, 8);
    return row;
}

static GtkWidget *make_section_label(const char *text)
{
    GtkWidget *label = gtk_label_new(text);
    gtk_widget_add_css_class(label, "title-4");
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_widget_set_margin_start(label, 4);
    gtk_widget_set_margin_top(label, 16);
    gtk_widget_set_margin_bottom(label, 6);
    return label;
}

GtkWidget *solock_dashboard_view_new(SolockApp *app)
{
    DashboardData *dd = g_new0(DashboardData, 1);
    dd->app = app;

    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll),
                                   GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);

    GtkWidget *clamp = adw_clamp_new();
    adw_clamp_set_maximum_size(ADW_CLAMP(clamp), 600);
    gtk_widget_set_margin_start(clamp, 16);
    gtk_widget_set_margin_end(clamp, 16);
    gtk_widget_set_margin_top(clamp, 8);
    gtk_widget_set_margin_bottom(clamp, 16);

    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);

    /* refresh button + spinner */
    GtkWidget *toolbar = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_halign(toolbar, GTK_ALIGN_END);
    gtk_widget_set_margin_bottom(toolbar, 8);

    dd->spinner = gtk_spinner_new();
    gtk_widget_set_visible(dd->spinner, FALSE);
    gtk_box_append(GTK_BOX(toolbar), dd->spinner);

    dd->refresh_btn = gtk_button_new_from_icon_name("view-refresh-symbolic");
    gtk_widget_add_css_class(dd->refresh_btn, "flat");
    g_signal_connect(dd->refresh_btn, "clicked", G_CALLBACK(on_refresh_clicked), dd);
    gtk_box_append(GTK_BOX(toolbar), dd->refresh_btn);

    gtk_box_append(GTK_BOX(box), toolbar);

    /* network indicator */
    GtkWidget *network_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_bottom(network_box, 8);

    GtkWidget *net_label = gtk_label_new("Network:");
    gtk_widget_add_css_class(net_label, "dim-label");
    gtk_box_append(GTK_BOX(network_box), net_label);

    dd->network_label = gtk_label_new("-");
    gtk_widget_add_css_class(dd->network_label, "caption");
    gtk_box_append(GTK_BOX(network_box), dd->network_label);

    gtk_box_append(GTK_BOX(box), network_box);

    /* account section */
    gtk_box_append(GTK_BOX(box), make_section_label("\xf0\x9f\x91\xa4 Account"));

    GtkWidget *account_group = adw_preferences_group_new();
    gtk_widget_set_margin_start(account_group, 8);
    gtk_widget_set_margin_end(account_group, 8);

    dd->deployer_row = make_action_row("Deployer Address", "-");
    GtkWidget *deployer_copy = gtk_button_new_from_icon_name("edit-copy-symbolic");
    gtk_widget_add_css_class(deployer_copy, "flat");
    gtk_widget_set_valign(deployer_copy, GTK_ALIGN_CENTER);
    g_signal_connect(deployer_copy, "clicked", G_CALLBACK(on_copy_deployer), dd);
    adw_action_row_add_suffix(ADW_ACTION_ROW(dd->deployer_row), deployer_copy);
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(account_group), dd->deployer_row);

    dd->program_row = make_action_row("Program ID", "-");
    GtkWidget *program_copy = gtk_button_new_from_icon_name("edit-copy-symbolic");
    gtk_widget_add_css_class(program_copy, "flat");
    gtk_widget_set_valign(program_copy, GTK_ALIGN_CENTER);
    g_signal_connect(program_copy, "clicked", G_CALLBACK(on_copy_program), dd);
    adw_action_row_add_suffix(ADW_ACTION_ROW(dd->program_row), program_copy);
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(account_group), dd->program_row);

    dd->balance_row = make_action_row("Balance", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(account_group), dd->balance_row);

    gtk_box_append(GTK_BOX(box), account_group);

    /* status section */
    gtk_box_append(GTK_BOX(box), make_section_label("\xf0\x9f\x93\xa1 Status"));

    GtkWidget *status_group = adw_preferences_group_new();
    gtk_widget_set_margin_start(status_group, 8);
    gtk_widget_set_margin_end(status_group, 8);

    dd->program_status_row = make_action_row("Program", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(status_group), dd->program_status_row);

    dd->vault_status_row = make_action_row("Vault", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(status_group), dd->vault_status_row);

    gtk_box_append(GTK_BOX(box), status_group);

    /* entries section */
    gtk_box_append(GTK_BOX(box), make_section_label("\xf0\x9f\x93\xa6 Entries"));

    GtkWidget *entries_group = adw_preferences_group_new();
    gtk_widget_set_margin_start(entries_group, 8);
    gtk_widget_set_margin_end(entries_group, 8);

    dd->entries_row = make_action_row("Total", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(entries_group), dd->entries_row);

    dd->passwords_row = make_action_row("Passwords", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(entries_group), dd->passwords_row);

    dd->notes_row = make_action_row("Notes", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(entries_group), dd->notes_row);

    dd->cards_row = make_action_row("Cards", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(entries_group), dd->cards_row);

    dd->totps_row = make_action_row("Authenticators", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(entries_group), dd->totps_row);

    dd->rent_row = make_action_row("Estimated Rent", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(entries_group), dd->rent_row);

    gtk_box_append(GTK_BOX(box), entries_group);

    /* sync section */
    gtk_box_append(GTK_BOX(box), make_section_label("\xf0\x9f\x94\x84 Sync"));

    GtkWidget *sync_group = adw_preferences_group_new();
    gtk_widget_set_margin_start(sync_group, 8);
    gtk_widget_set_margin_end(sync_group, 8);

    dd->sync_row = make_action_row("Last Sync", "-");
    adw_preferences_group_add(ADW_PREFERENCES_GROUP(sync_group), dd->sync_row);

    gtk_box_append(GTK_BOX(box), sync_group);

    GtkWidget *clear_btn = gtk_button_new_with_label("Clear Local DB");
    gtk_widget_add_css_class(clear_btn, "flat");
    gtk_widget_set_halign(clear_btn, GTK_ALIGN_START);
    gtk_widget_set_margin_top(clear_btn, 16);
    g_signal_connect(clear_btn, "clicked", G_CALLBACK(on_clear_local_db_clicked), dd);
    gtk_box_append(GTK_BOX(box), clear_btn);

    adw_clamp_set_child(ADW_CLAMP(clamp), box);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), clamp);

    dashboard_refresh(dd);

    return scroll;
}

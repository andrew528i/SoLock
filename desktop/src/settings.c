#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);

typedef struct {
    SolockApp *app;
    GtkWidget *timeout_spin;
    GtkWidget *clipboard_spin;
    GtkWidget *paste_dropdown;
    GtkWidget *network_dropdown;
} SettingsData;

static void on_settings_changed(GtkWidget *widget, gpointer data)
{
    (void)widget;
    SettingsData *sd = data;
    SolockConfig *config = solock_app_get_config(sd->app);

    solock_config_set_timeout_minutes(config,
        (int)gtk_spin_button_get_value(GTK_SPIN_BUTTON(sd->timeout_spin)));
    solock_config_set_clipboard_clear_seconds(config,
        (int)gtk_spin_button_get_value(GTK_SPIN_BUTTON(sd->clipboard_spin)));

    guint selected = gtk_drop_down_get_selected(GTK_DROP_DOWN(sd->paste_dropdown));
    solock_config_set_paste_method(config, selected == 0 ? "wtype" : "clipboard");

    solock_config_save(config);
}

static void on_network_changed(GtkDropDown *dropdown, GParamSpec *pspec, gpointer data)
{
    (void)pspec;
    SettingsData *sd = data;
    guint idx = gtk_drop_down_get_selected(dropdown);
    const char *network = (idx == 0) ? "devnet" : "mainnet-beta";
    const char *rpc = (idx == 0)
        ? "https://api.devnet.solana.com"
        : "https://api.mainnet-beta.solana.com";

    SolockClient *client = solock_app_get_client(sd->app);
    GError *error = NULL;

    JsonBuilder *b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "key");
    json_builder_add_string_value(b, "network");
    json_builder_set_member_name(b, "value");
    json_builder_add_string_value(b, network);
    json_builder_end_object(b);
    JsonNode *params = json_builder_get_root(b);
    JsonNode *result = solock_client_call(client, "set_config", params, &error);
    if (result) json_node_unref(result);
    if (error) g_error_free(error);
    json_node_unref(params);
    g_object_unref(b);

    b = json_builder_new();
    json_builder_begin_object(b);
    json_builder_set_member_name(b, "key");
    json_builder_add_string_value(b, "rpc_url");
    json_builder_set_member_name(b, "value");
    json_builder_add_string_value(b, rpc);
    json_builder_end_object(b);
    params = json_builder_get_root(b);
    result = solock_client_call(client, "set_config", params, &error);
    if (result) json_node_unref(result);
    if (error) g_error_free(error);
    json_node_unref(params);
    g_object_unref(b);
}

static GtkWidget *settings_row(const char *label, GtkWidget *control)
{
    GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 12);
    gtk_widget_set_margin_start(row, 16);
    gtk_widget_set_margin_end(row, 16);
    gtk_widget_set_margin_top(row, 8);

    GtkWidget *lbl = gtk_label_new(label);
    gtk_widget_set_size_request(lbl, 220, -1);
    gtk_label_set_xalign(GTK_LABEL(lbl), 0);
    gtk_box_append(GTK_BOX(row), lbl);

    gtk_widget_set_hexpand(control, TRUE);
    gtk_box_append(GTK_BOX(row), control);

    return row;
}

GtkWidget *solock_settings_view_new(SolockApp *app)
{
    SolockConfig *config = solock_app_get_config(app);

    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 4);
    gtk_widget_set_margin_top(box, 16);
    gtk_widget_set_margin_bottom(box, 16);

    GtkWidget *title = gtk_label_new("\xe2\x9a\x99 Settings");
    gtk_widget_add_css_class(title, "title-2");
    gtk_widget_set_margin_start(title, 16);
    gtk_widget_set_margin_bottom(title, 8);
    gtk_label_set_xalign(GTK_LABEL(title), 0);
    gtk_box_append(GTK_BOX(box), title);

    SettingsData *sd = g_new0(SettingsData, 1);
    sd->app = app;

    sd->timeout_spin = gtk_spin_button_new_with_range(1, 1440, 10);
    gtk_spin_button_set_value(GTK_SPIN_BUTTON(sd->timeout_spin),
                               solock_config_get_timeout_minutes(config));
    gtk_box_append(GTK_BOX(box), settings_row("\xf0\x9f\x94\x92 Session timeout (min)", sd->timeout_spin));

    sd->clipboard_spin = gtk_spin_button_new_with_range(0, 120, 5);
    gtk_spin_button_set_value(GTK_SPIN_BUTTON(sd->clipboard_spin),
                               solock_config_get_clipboard_clear_seconds(config));
    gtk_box_append(GTK_BOX(box), settings_row("\xf0\x9f\x93\x8b Clear clipboard (sec)", sd->clipboard_spin));

    const char *paste_options[] = { "wtype (auto-type)", "clipboard", NULL };
    sd->paste_dropdown = gtk_drop_down_new_from_strings(paste_options);
    const char *current_method = solock_config_get_paste_method(config);
    gtk_drop_down_set_selected(GTK_DROP_DOWN(sd->paste_dropdown),
                                g_strcmp0(current_method, "wtype") == 0 ? 0 : 1);
    gtk_box_append(GTK_BOX(box), settings_row("\xe2\x8c\xa8 Paste method", sd->paste_dropdown));

    GtkWidget *sep = gtk_separator_new(GTK_ORIENTATION_HORIZONTAL);
    gtk_widget_set_margin_top(sep, 12);
    gtk_widget_set_margin_bottom(sep, 4);
    gtk_widget_set_margin_start(sep, 16);
    gtk_widget_set_margin_end(sep, 16);
    gtk_box_append(GTK_BOX(box), sep);

    const char *network_options[] = { "devnet", "mainnet-beta", NULL };
    sd->network_dropdown = gtk_drop_down_new_from_strings(network_options);
    gtk_drop_down_set_selected(GTK_DROP_DOWN(sd->network_dropdown), 0);
    gtk_box_append(GTK_BOX(box), settings_row("\xf0\x9f\x8c\x90 Network", sd->network_dropdown));

    g_signal_connect(sd->timeout_spin, "value-changed", G_CALLBACK(on_settings_changed), sd);
    g_signal_connect(sd->clipboard_spin, "value-changed", G_CALLBACK(on_settings_changed), sd);
    g_signal_connect(sd->paste_dropdown, "notify::selected", G_CALLBACK(on_settings_changed), sd);
    g_signal_connect(sd->network_dropdown, "notify::selected", G_CALLBACK(on_network_changed), sd);

    return box;
}

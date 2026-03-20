#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);

typedef struct {
    SolockApp *app;
    GtkWidget *deployer_label;
    GtkWidget *balance_label;
    GtkWidget *program_label;
    GtkWidget *program_status;
    GtkWidget *vault_status;
    GtkWidget *entries_label;
    GtkWidget *network_label;
    GtkWidget *sync_label;
} DashboardData;

static void dashboard_refresh(DashboardData *dd)
{
    SolockClient *client = solock_app_get_client(dd->app);
    if (solock_client_is_locked(client)) return;

    GError *error = NULL;
    JsonNode *info = solock_client_get_dashboard(client, &error);
    if (!info) return;

    JsonObject *obj = json_node_get_object(info);

    gtk_label_set_text(GTK_LABEL(dd->deployer_label),
                       json_object_get_string_member(obj, "deployer_address"));

    gint64 balance = json_object_get_int_member(obj, "balance");
    char *bal_str = g_strdup_printf("%.4f SOL", (double)balance / 1e9);
    gtk_label_set_text(GTK_LABEL(dd->balance_label), bal_str);
    g_free(bal_str);

    gtk_label_set_text(GTK_LABEL(dd->program_label),
                       json_object_get_string_member(obj, "program_id"));

    gboolean deployed = json_object_get_boolean_member(obj, "program_deployed");
    gtk_label_set_text(GTK_LABEL(dd->program_status), deployed ? "deployed" : "not deployed");

    gboolean vault = json_object_get_boolean_member(obj, "vault_exists");
    gtk_label_set_text(GTK_LABEL(dd->vault_status), vault ? "ready" : "not initialized");

    gint64 count = json_object_get_int_member(obj, "entry_count");
    char *entries_str = g_strdup_printf("%lld", (long long)count);
    gtk_label_set_text(GTK_LABEL(dd->entries_label), entries_str);
    g_free(entries_str);

    const char *network = json_object_get_string_member(obj, "network");
    if (network && *network)
        gtk_label_set_text(GTK_LABEL(dd->network_label), network);

    json_node_unref(info);
}

static GtkWidget *info_row(const char *label, GtkWidget **value_out)
{
    GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 12);
    gtk_widget_set_margin_start(row, 16);
    gtk_widget_set_margin_end(row, 16);
    gtk_widget_set_margin_top(row, 4);

    GtkWidget *lbl = gtk_label_new(label);
    gtk_widget_add_css_class(lbl, "dim-label");
    gtk_widget_set_size_request(lbl, 120, -1);
    gtk_label_set_xalign(GTK_LABEL(lbl), 0);
    gtk_box_append(GTK_BOX(row), lbl);

    GtkWidget *val = gtk_label_new("-");
    gtk_label_set_xalign(GTK_LABEL(val), 0);
    gtk_label_set_selectable(GTK_LABEL(val), TRUE);
    gtk_label_set_ellipsize(GTK_LABEL(val), PANGO_ELLIPSIZE_MIDDLE);
    gtk_widget_set_hexpand(val, TRUE);
    gtk_box_append(GTK_BOX(row), val);

    *value_out = val;
    return row;
}

GtkWidget *solock_dashboard_view_new(SolockApp *app)
{
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 8);
    gtk_widget_set_margin_top(box, 16);
    gtk_widget_set_margin_bottom(box, 16);

    GtkWidget *title = gtk_label_new("Dashboard");
    gtk_widget_add_css_class(title, "title-2");
    gtk_widget_set_margin_start(title, 16);
    gtk_label_set_xalign(GTK_LABEL(title), 0);
    gtk_box_append(GTK_BOX(box), title);

    DashboardData *dd = g_new0(DashboardData, 1);
    dd->app = app;

    gtk_box_append(GTK_BOX(box), info_row("Deployer", &dd->deployer_label));
    gtk_box_append(GTK_BOX(box), info_row("Balance", &dd->balance_label));
    gtk_box_append(GTK_BOX(box), info_row("Program", &dd->program_label));
    gtk_box_append(GTK_BOX(box), info_row("Status", &dd->program_status));
    gtk_box_append(GTK_BOX(box), info_row("Vault", &dd->vault_status));
    gtk_box_append(GTK_BOX(box), info_row("Entries", &dd->entries_label));
    gtk_box_append(GTK_BOX(box), info_row("Network", &dd->network_label));
    gtk_box_append(GTK_BOX(box), info_row("Last Sync", &dd->sync_label));

    GtkWidget *refresh_btn = gtk_button_new_with_label("Refresh");
    gtk_widget_set_margin_start(refresh_btn, 16);
    gtk_widget_set_margin_top(refresh_btn, 12);
    gtk_widget_set_halign(refresh_btn, GTK_ALIGN_START);
    g_signal_connect_swapped(refresh_btn, "clicked", G_CALLBACK(dashboard_refresh), dd);
    gtk_box_append(GTK_BOX(box), refresh_btn);

    dashboard_refresh(dd);

    return box;
}

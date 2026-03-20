#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);
extern void          solock_app_show_main_window(SolockApp *app);

#define SNI_DBUS_PATH "/StatusNotifierItem"
#define SNI_DBUS_IFACE "org.kde.StatusNotifierItem"
#define SNW_DBUS_NAME "org.kde.StatusNotifierWatcher"
#define SNW_DBUS_PATH "/StatusNotifierWatcher"
#define SNW_DBUS_IFACE "org.kde.StatusNotifierWatcher"
#define DBUSMENU_PATH "/MenuBar"
#define DBUSMENU_IFACE "com.canonical.dbusmenu"

enum {
    MENU_ID_ROOT = 0,
    MENU_ID_SHOW = 1,
    MENU_ID_SEP1 = 2,
    MENU_ID_STATUS = 3,
    MENU_ID_LOCK = 4,
    MENU_ID_MANAGE = 5,
    MENU_ID_SEP2 = 6,
    MENU_ID_QUIT = 7,
};

typedef struct {
    SolockApp       *app;
    GDBusConnection *conn;
    guint            sni_reg_id;
    guint            menu_reg_id;
    guint            bus_name_id;
    char            *bus_name;
    gboolean         locked;
    guint32          menu_revision;
} TrayData;

static TrayData *tray = NULL;

static const char *sni_introspection_xml =
    "<node>"
    "  <interface name='" SNI_DBUS_IFACE "'>"
    "    <method name='Activate'>"
    "      <arg name='x' type='i' direction='in'/>"
    "      <arg name='y' type='i' direction='in'/>"
    "    </method>"
    "    <method name='ContextMenu'>"
    "      <arg name='x' type='i' direction='in'/>"
    "      <arg name='y' type='i' direction='in'/>"
    "    </method>"
    "    <method name='SecondaryActivate'>"
    "      <arg name='x' type='i' direction='in'/>"
    "      <arg name='y' type='i' direction='in'/>"
    "    </method>"
    "    <method name='Scroll'>"
    "      <arg name='delta' type='i' direction='in'/>"
    "      <arg name='orientation' type='s' direction='in'/>"
    "    </method>"
    "    <property name='Category' type='s' access='read'/>"
    "    <property name='Id' type='s' access='read'/>"
    "    <property name='Title' type='s' access='read'/>"
    "    <property name='Status' type='s' access='read'/>"
    "    <property name='IconName' type='s' access='read'/>"
    "    <property name='IconThemePath' type='s' access='read'/>"
    "    <property name='Menu' type='o' access='read'/>"
    "    <property name='ItemIsMenu' type='b' access='read'/>"
    "    <property name='ToolTip' type='(sa(iiay)ss)' access='read'/>"
    "    <signal name='NewTitle'/>"
    "    <signal name='NewIcon'/>"
    "    <signal name='NewStatus'>"
    "      <arg name='status' type='s'/>"
    "    </signal>"
    "    <signal name='NewToolTip'/>"
    "  </interface>"
    "</node>";

static const char *dbusmenu_introspection_xml =
    "<node>"
    "  <interface name='" DBUSMENU_IFACE "'>"
    "    <method name='GetLayout'>"
    "      <arg name='parentId' type='i' direction='in'/>"
    "      <arg name='recursionDepth' type='i' direction='in'/>"
    "      <arg name='propertyNames' type='as' direction='in'/>"
    "      <arg name='revision' type='u' direction='out'/>"
    "      <arg name='layout' type='(ia{sv}av)' direction='out'/>"
    "    </method>"
    "    <method name='GetGroupProperties'>"
    "      <arg name='ids' type='ai' direction='in'/>"
    "      <arg name='propertyNames' type='as' direction='in'/>"
    "      <arg name='properties' type='a(ia{sv})' direction='out'/>"
    "    </method>"
    "    <method name='GetProperty'>"
    "      <arg name='id' type='i' direction='in'/>"
    "      <arg name='name' type='s' direction='in'/>"
    "      <arg name='value' type='v' direction='out'/>"
    "    </method>"
    "    <method name='Event'>"
    "      <arg name='id' type='i' direction='in'/>"
    "      <arg name='eventId' type='s' direction='in'/>"
    "      <arg name='data' type='v' direction='in'/>"
    "      <arg name='timestamp' type='u' direction='in'/>"
    "    </method>"
    "    <method name='AboutToShow'>"
    "      <arg name='id' type='i' direction='in'/>"
    "      <arg name='needUpdate' type='b' direction='out'/>"
    "    </method>"
    "    <method name='EventGroup'>"
    "      <arg name='events' type='a(isvu)' direction='in'/>"
    "      <arg name='idErrors' type='ai' direction='out'/>"
    "    </method>"
    "    <method name='AboutToShowGroup'>"
    "      <arg name='ids' type='ai' direction='in'/>"
    "      <arg name='updatesNeeded' type='ai' direction='out'/>"
    "      <arg name='idErrors' type='ai' direction='out'/>"
    "    </method>"
    "    <signal name='ItemsPropertiesUpdated'>"
    "      <arg name='updatedProps' type='a(ia{sv})'/>"
    "      <arg name='removedProps' type='a(ias)'/>"
    "    </signal>"
    "    <signal name='LayoutUpdated'>"
    "      <arg name='revision' type='u'/>"
    "      <arg name='parent' type='i'/>"
    "    </signal>"
    "    <property name='Version' type='u' access='read'/>"
    "    <property name='TextDirection' type='s' access='read'/>"
    "    <property name='Status' type='s' access='read'/>"
    "    <property name='IconThemePath' type='as' access='read'/>"
    "  </interface>"
    "</node>";

static GVariant *build_menu_item_variant(int id, const char *label,
                                          const char *item_type,
                                          gboolean enabled, gboolean visible)
{
    GVariantBuilder props;
    g_variant_builder_init(&props, G_VARIANT_TYPE("a{sv}"));

    if (label)
        g_variant_builder_add(&props, "{sv}", "label", g_variant_new_string(label));

    if (item_type)
        g_variant_builder_add(&props, "{sv}", "type", g_variant_new_string(item_type));

    g_variant_builder_add(&props, "{sv}", "enabled", g_variant_new_boolean(enabled));
    g_variant_builder_add(&props, "{sv}", "visible", g_variant_new_boolean(visible));

    GVariantBuilder empty_children;
    g_variant_builder_init(&empty_children, G_VARIANT_TYPE("av"));

    return g_variant_new("(ia{sv}av)", id, &props, &empty_children);
}

static GVariant *build_menu_layout(void)
{
    GVariantBuilder root_props;
    g_variant_builder_init(&root_props, G_VARIANT_TYPE("a{sv}"));
    g_variant_builder_add(&root_props, "{sv}", "children-display",
                          g_variant_new_string("submenu"));

    GVariantBuilder children;
    g_variant_builder_init(&children, G_VARIANT_TYPE("av"));

    g_variant_builder_add(&children, "v",
        build_menu_item_variant(MENU_ID_SHOW, "Show Vault", NULL, TRUE, TRUE));

    g_variant_builder_add(&children, "v",
        build_menu_item_variant(MENU_ID_SEP1, NULL, "separator", TRUE, TRUE));

    const char *status_text = tray->locked ? "Locked" : "Unlocked";
    g_variant_builder_add(&children, "v",
        build_menu_item_variant(MENU_ID_STATUS, status_text, NULL, FALSE, TRUE));

    const char *lock_label = tray->locked ? "Unlock" : "Lock";
    g_variant_builder_add(&children, "v",
        build_menu_item_variant(MENU_ID_LOCK, lock_label, NULL, TRUE, TRUE));

    g_variant_builder_add(&children, "v",
        build_menu_item_variant(MENU_ID_MANAGE, "Manage Vault", NULL, TRUE, TRUE));

    g_variant_builder_add(&children, "v",
        build_menu_item_variant(MENU_ID_SEP2, NULL, "separator", TRUE, TRUE));

    g_variant_builder_add(&children, "v",
        build_menu_item_variant(MENU_ID_QUIT, "Quit", NULL, TRUE, TRUE));

    return g_variant_new("(ia{sv}av)", MENU_ID_ROOT, &root_props, &children);
}

static void handle_menu_event(int id)
{
    switch (id) {
    case MENU_ID_SHOW:
        solock_popup_toggle(solock_app_get_popup(tray->app));
        break;
    case MENU_ID_LOCK:
        if (tray->locked) {
            solock_popup_show(solock_app_get_popup(tray->app));
        } else {
            solock_client_lock(solock_app_get_client(tray->app));
            solock_tray_update_status(tray->app, TRUE);
        }
        break;
    case MENU_ID_MANAGE:
        solock_app_show_main_window(tray->app);
        break;
    case MENU_ID_QUIT:
        g_application_quit(G_APPLICATION(tray->app));
        break;
    }
}

static void sni_method_call(GDBusConnection *conn, const char *sender,
                             const char *path, const char *iface,
                             const char *method, GVariant *params,
                             GDBusMethodInvocation *invocation, gpointer data)
{
    (void)conn; (void)sender; (void)path; (void)iface; (void)params; (void)data;

    if (g_strcmp0(method, "Activate") == 0) {
        solock_popup_toggle(solock_app_get_popup(tray->app));
        g_dbus_method_invocation_return_value(invocation, NULL);
    } else if (g_strcmp0(method, "ContextMenu") == 0 ||
               g_strcmp0(method, "SecondaryActivate") == 0 ||
               g_strcmp0(method, "Scroll") == 0) {
        g_dbus_method_invocation_return_value(invocation, NULL);
    } else {
        g_dbus_method_invocation_return_dbus_error(invocation,
            "org.freedesktop.DBus.Error.UnknownMethod", "Unknown method");
    }
}

static GVariant *sni_get_property(GDBusConnection *conn, const char *sender,
                                   const char *path, const char *iface,
                                   const char *property, GError **error,
                                   gpointer data)
{
    (void)conn; (void)sender; (void)path; (void)iface; (void)error; (void)data;

    if (g_strcmp0(property, "Category") == 0)
        return g_variant_new_string("ApplicationStatus");
    if (g_strcmp0(property, "Id") == 0)
        return g_variant_new_string("solock-desktop");
    if (g_strcmp0(property, "Title") == 0)
        return g_variant_new_string("SoLock");
    if (g_strcmp0(property, "Status") == 0)
        return g_variant_new_string("Active");
    if (g_strcmp0(property, "IconName") == 0)
        return g_variant_new_string(tray->locked ? "security-low-symbolic" : "security-high-symbolic");
    if (g_strcmp0(property, "IconThemePath") == 0)
        return g_variant_new_string("");
    if (g_strcmp0(property, "Menu") == 0)
        return g_variant_new_object_path(DBUSMENU_PATH);
    if (g_strcmp0(property, "ItemIsMenu") == 0)
        return g_variant_new_boolean(TRUE);
    if (g_strcmp0(property, "ToolTip") == 0) {
        const char *tooltip = tray->locked ? "SoLock - Locked" : "SoLock - Unlocked";
        GVariantBuilder pixmaps;
        g_variant_builder_init(&pixmaps, G_VARIANT_TYPE("a(iiay)"));
        return g_variant_new("(sa(iiay)ss)", "", &pixmaps, "SoLock", tooltip);
    }
    return NULL;
}

static void menu_method_call(GDBusConnection *conn, const char *sender,
                              const char *path, const char *iface,
                              const char *method, GVariant *params,
                              GDBusMethodInvocation *invocation, gpointer data)
{
    (void)conn; (void)sender; (void)path; (void)iface; (void)data;

    if (g_strcmp0(method, "GetLayout") == 0) {
        GVariant *layout = build_menu_layout();
        g_dbus_method_invocation_return_value(invocation,
            g_variant_new("(u@(ia{sv}av))", tray->menu_revision, layout));
    } else if (g_strcmp0(method, "Event") == 0) {
        gint32 id;
        const char *event_id;
        GVariant *event_data;
        guint32 timestamp;
        g_variant_get(params, "(is@vu)", &id, &event_id, &event_data, &timestamp);

        if (g_strcmp0(event_id, "clicked") == 0) {
            handle_menu_event(id);
        }

        g_variant_unref(event_data);
        g_dbus_method_invocation_return_value(invocation, NULL);
    } else if (g_strcmp0(method, "EventGroup") == 0) {
        GVariantIter *iter;
        g_variant_get(params, "(a(isvu))", &iter);

        gint32 id;
        const char *event_id;
        GVariant *event_data;
        guint32 timestamp;

        while (g_variant_iter_next(iter, "(is@vu)", &id, &event_id, &event_data, &timestamp)) {
            if (g_strcmp0(event_id, "clicked") == 0) {
                handle_menu_event(id);
            }
            g_variant_unref(event_data);
        }
        g_variant_iter_free(iter);

        GVariantBuilder errors;
        g_variant_builder_init(&errors, G_VARIANT_TYPE("ai"));
        g_dbus_method_invocation_return_value(invocation,
            g_variant_new("(ai)", &errors));
    } else if (g_strcmp0(method, "AboutToShow") == 0) {
        g_dbus_method_invocation_return_value(invocation,
            g_variant_new("(b)", TRUE));
    } else if (g_strcmp0(method, "AboutToShowGroup") == 0) {
        GVariantBuilder updates;
        g_variant_builder_init(&updates, G_VARIANT_TYPE("ai"));

        GVariantBuilder errors;
        g_variant_builder_init(&errors, G_VARIANT_TYPE("ai"));

        g_dbus_method_invocation_return_value(invocation,
            g_variant_new("(aiai)", &updates, &errors));
    } else if (g_strcmp0(method, "GetGroupProperties") == 0) {
        GVariantBuilder b;
        g_variant_builder_init(&b, G_VARIANT_TYPE("a(ia{sv})"));
        g_dbus_method_invocation_return_value(invocation,
            g_variant_new("(a(ia{sv}))", &b));
    } else if (g_strcmp0(method, "GetProperty") == 0) {
        g_dbus_method_invocation_return_value(invocation,
            g_variant_new("(v)", g_variant_new_string("")));
    } else {
        g_dbus_method_invocation_return_dbus_error(invocation,
            "org.freedesktop.DBus.Error.UnknownMethod", "Unknown method");
    }
}

static GVariant *menu_get_property(GDBusConnection *conn, const char *sender,
                                    const char *path, const char *iface,
                                    const char *property, GError **error,
                                    gpointer data)
{
    (void)conn; (void)sender; (void)path; (void)iface; (void)error; (void)data;

    if (g_strcmp0(property, "Version") == 0)
        return g_variant_new_uint32(3);
    if (g_strcmp0(property, "TextDirection") == 0)
        return g_variant_new_string("ltr");
    if (g_strcmp0(property, "Status") == 0)
        return g_variant_new_string("normal");
    if (g_strcmp0(property, "IconThemePath") == 0) {
        GVariantBuilder b;
        g_variant_builder_init(&b, G_VARIANT_TYPE("as"));
        return g_variant_new("as", &b);
    }
    return NULL;
}

static void register_with_watcher(GDBusConnection *conn, const char *bus_name)
{
    g_dbus_connection_call(conn,
        SNW_DBUS_NAME, SNW_DBUS_PATH, SNW_DBUS_IFACE,
        "RegisterStatusNotifierItem",
        g_variant_new("(s)", bus_name),
        NULL, G_DBUS_CALL_FLAGS_NONE, -1, NULL, NULL, NULL);
}

static void on_bus_acquired(GDBusConnection *conn, const char *name, gpointer data)
{
    (void)data;
    tray->conn = conn;
    tray->bus_name = g_strdup(name);

    GError *error = NULL;

    GDBusNodeInfo *sni_info = g_dbus_node_info_new_for_xml(sni_introspection_xml, &error);
    if (!sni_info) {
        g_warning("Failed to parse SNI introspection: %s", error->message);
        g_error_free(error);
        return;
    }

    static const GDBusInterfaceVTable sni_vtable = {
        .method_call = sni_method_call,
        .get_property = sni_get_property,
    };

    tray->sni_reg_id = g_dbus_connection_register_object(conn,
        SNI_DBUS_PATH, sni_info->interfaces[0], &sni_vtable, NULL, NULL, &error);
    g_dbus_node_info_unref(sni_info);

    if (!tray->sni_reg_id) {
        g_warning("Failed to register SNI object: %s", error->message);
        g_error_free(error);
        return;
    }

    GDBusNodeInfo *menu_info = g_dbus_node_info_new_for_xml(dbusmenu_introspection_xml, &error);
    if (!menu_info) {
        g_warning("Failed to parse DBusMenu introspection: %s", error->message);
        g_error_free(error);
        return;
    }

    static const GDBusInterfaceVTable menu_vtable = {
        .method_call = menu_method_call,
        .get_property = menu_get_property,
    };

    tray->menu_reg_id = g_dbus_connection_register_object(conn,
        DBUSMENU_PATH, menu_info->interfaces[0], &menu_vtable, NULL, NULL, &error);
    g_dbus_node_info_unref(menu_info);

    if (!tray->menu_reg_id) {
        g_warning("Failed to register DBusMenu object: %s", error->message);
        g_error_free(error);
        return;
    }

    register_with_watcher(conn, name);
}

static void on_name_acquired(GDBusConnection *conn, const char *name, gpointer data)
{
    (void)conn; (void)name; (void)data;
}

static void on_name_lost(GDBusConnection *conn, const char *name, gpointer data)
{
    (void)conn; (void)name; (void)data;
}

void solock_tray_setup(SolockApp *app)
{
    tray = g_new0(TrayData, 1);
    tray->app = app;
    tray->locked = TRUE;
    tray->menu_revision = 1;

    tray->bus_name_id = g_bus_own_name(G_BUS_TYPE_SESSION,
        "org.kde.StatusNotifierItem-solock",
        G_BUS_NAME_OWNER_FLAGS_NONE,
        on_bus_acquired, on_name_acquired, on_name_lost,
        NULL, NULL);
}

void solock_tray_update_status(SolockApp *app, gboolean locked)
{
    (void)app;
    if (!tray || !tray->conn) return;

    tray->locked = locked;
    tray->menu_revision++;

    g_dbus_connection_emit_signal(tray->conn, NULL, SNI_DBUS_PATH,
        SNI_DBUS_IFACE, "NewIcon", NULL, NULL);

    g_dbus_connection_emit_signal(tray->conn, NULL, SNI_DBUS_PATH,
        SNI_DBUS_IFACE, "NewToolTip", NULL, NULL);

    g_dbus_connection_emit_signal(tray->conn, NULL, DBUSMENU_PATH,
        DBUSMENU_IFACE, "LayoutUpdated",
        g_variant_new("(ui)", tray->menu_revision, 0), NULL);
}

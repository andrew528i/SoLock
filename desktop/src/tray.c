#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);
extern void          solock_app_show_main_window(SolockApp *app);

void solock_tray_setup(SolockApp *app)
{
    (void)app;
    /* StatusNotifierItem setup is desktop-environment specific.
       On GNOME/KDE/hyprland with waybar, this is typically done via
       libappindicator or through D-Bus StatusNotifierItem protocol.
       For initial implementation, tray is managed externally
       (e.g. waybar module or manual dbus registration). */
}

void solock_tray_update_status(SolockApp *app, gboolean locked)
{
    (void)app;
    (void)locked;
    /* Update tray icon/tooltip to reflect lock state */
}

#include "solock-desktop.h"

gboolean solock_wtype_available(void)
{
    char *path = g_find_program_in_path("wtype");
    if (path) {
        g_free(path);
        return TRUE;
    }
    return FALSE;
}

gboolean solock_wtype_type(const char *text, GError **error)
{
    char *argv[] = { "wtype", "--", (char *)text, NULL };
    gint status;
    gboolean ok = g_spawn_sync(NULL, argv, NULL,
                                G_SPAWN_SEARCH_PATH | G_SPAWN_STDOUT_TO_DEV_NULL | G_SPAWN_STDERR_TO_DEV_NULL,
                                NULL, NULL, NULL, NULL, &status, error);
    if (!ok) return FALSE;
    if (!g_spawn_check_wait_status(status, error)) return FALSE;
    return TRUE;
}

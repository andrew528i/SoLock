#include "solock-desktop.h"

static guint clear_timer_id = 0;

static gboolean clear_clipboard_cb(gpointer data)
{
    (void)data;
    char *argv[] = { "wl-copy", "--clear", NULL };
    g_spawn_sync(NULL, argv, NULL,
                 G_SPAWN_SEARCH_PATH | G_SPAWN_STDOUT_TO_DEV_NULL | G_SPAWN_STDERR_TO_DEV_NULL,
                 NULL, NULL, NULL, NULL, NULL, NULL);
    clear_timer_id = 0;
    return G_SOURCE_REMOVE;
}

gboolean solock_clipboard_copy(const char *text, int clear_after_seconds, GError **error)
{
    if (clear_timer_id > 0) {
        g_source_remove(clear_timer_id);
        clear_timer_id = 0;
    }

    char *argv[] = { "wl-copy", "--", (char *)text, NULL };
    gint status;
    gboolean ok = g_spawn_sync(NULL, argv, NULL,
                                G_SPAWN_SEARCH_PATH | G_SPAWN_STDOUT_TO_DEV_NULL | G_SPAWN_STDERR_TO_DEV_NULL,
                                NULL, NULL, NULL, NULL, &status, error);
    if (!ok) return FALSE;
    if (!g_spawn_check_wait_status(status, error)) return FALSE;

    if (clear_after_seconds > 0) {
        clear_timer_id = g_timeout_add_seconds(clear_after_seconds, clear_clipboard_cb, NULL);
    }

    return TRUE;
}

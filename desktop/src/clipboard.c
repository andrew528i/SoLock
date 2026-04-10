#include "solock-desktop.h"
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <signal.h>
#include <sys/wait.h>

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

static gboolean write_all(int fd, const char *buf, gsize len, GError **error)
{
    while (len > 0) {
        gssize n = write(fd, buf, len);
        if (n < 0) {
            if (errno == EINTR) continue;
            g_set_error(error, G_IO_ERROR, g_io_error_from_errno(errno),
                        "write to wl-copy: %s", g_strerror(errno));
            return FALSE;
        }
        buf += n;
        len -= (gsize)n;
    }
    return TRUE;
}

gboolean solock_clipboard_copy(const char *text, int clear_after_seconds, GError **error)
{
    if (clear_timer_id > 0) {
        g_source_remove(clear_timer_id);
        clear_timer_id = 0;
    }

    char *argv[] = { "wl-copy", NULL };
    GPid pid;
    gint stdin_fd;

    if (!g_spawn_async_with_pipes(NULL, argv, NULL,
                                   G_SPAWN_SEARCH_PATH | G_SPAWN_DO_NOT_REAP_CHILD |
                                   G_SPAWN_STDOUT_TO_DEV_NULL | G_SPAWN_STDERR_TO_DEV_NULL,
                                   NULL, NULL, &pid,
                                   &stdin_fd, NULL, NULL, error)) {
        return FALSE;
    }

    gsize len = strlen(text);
    gboolean ok = write_all(stdin_fd, text, len, error);
    close(stdin_fd);

    if (!ok) {
        kill(pid, SIGTERM);
        g_spawn_close_pid(pid);
        return FALSE;
    }

    gint status;
    if (waitpid(pid, &status, 0) < 0) {
        g_set_error(error, G_IO_ERROR, g_io_error_from_errno(errno),
                    "waitpid wl-copy: %s", g_strerror(errno));
        g_spawn_close_pid(pid);
        return FALSE;
    }
    g_spawn_close_pid(pid);

    if (!g_spawn_check_wait_status(status, error)) return FALSE;

    if (clear_after_seconds > 0) {
        clear_timer_id = g_timeout_add_seconds(clear_after_seconds, clear_clipboard_cb, NULL);
    }

    return TRUE;
}

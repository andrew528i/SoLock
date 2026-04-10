#include "solock-desktop.h"
#include <string.h>

struct _SolockConfig {
    int   timeout_minutes;
    int   clipboard_clear_seconds;
    char *paste_method;
    char *path;
};

static char *config_path(void)
{
    const char *home = g_get_home_dir();
    return g_build_filename(home, ".solock", "desktop.toml", NULL);
}

SolockConfig *solock_config_new(void)
{
    SolockConfig *c = g_new0(SolockConfig, 1);
    c->timeout_minutes = 720;
    c->clipboard_clear_seconds = 15;
    c->paste_method = g_strdup("wtype");
    c->path = config_path();
    return c;
}

void solock_config_free(SolockConfig *c)
{
    g_free(c->paste_method);
    g_free(c->path);
    g_free(c);
}

static char *read_file_contents(const char *path)
{
    char *contents = NULL;
    g_file_get_contents(path, &contents, NULL, NULL);
    return contents;
}

static int parse_int_value(const char *contents, const char *key, int default_val)
{
    char *pattern = g_strdup_printf("%s = ", key);
    char *found = strstr(contents, pattern);
    g_free(pattern);
    if (!found) return default_val;

    found += strlen(key) + 3;
    return atoi(found);
}

static char *parse_string_value(const char *contents, const char *key, const char *default_val)
{
    char *pattern = g_strdup_printf("%s = \"", key);
    char *found = strstr(contents, pattern);
    g_free(pattern);
    if (!found) return g_strdup(default_val);

    found += strlen(key) + 4;
    char *end = strchr(found, '"');
    if (!end) return g_strdup(default_val);

    return g_strndup(found, end - found);
}

gboolean solock_config_load(SolockConfig *c)
{
    char *contents = read_file_contents(c->path);
    if (!contents) return FALSE;

    c->timeout_minutes = parse_int_value(contents, "timeout_minutes", 720);
    c->clipboard_clear_seconds = parse_int_value(contents, "clear_after_seconds", 15);
    g_free(c->paste_method);
    c->paste_method = parse_string_value(contents, "method", "wtype");

    g_free(contents);
    return TRUE;
}

gboolean solock_config_save(SolockConfig *c)
{
    char *dir = g_path_get_dirname(c->path);
    g_mkdir_with_parents(dir, 0700);
    g_free(dir);

    GString *s = g_string_new("");
    g_string_append(s, "[session]\n");
    g_string_append_printf(s, "timeout_minutes = %d\n", c->timeout_minutes);
    g_string_append(s, "\n[clipboard]\n");
    g_string_append_printf(s, "clear_after_seconds = %d\n", c->clipboard_clear_seconds);
    g_string_append(s, "\n[paste]\n");
    g_string_append_printf(s, "method = \"%s\"\n", c->paste_method);

    gboolean ok = g_file_set_contents(c->path, s->str, s->len, NULL);
    g_string_free(s, TRUE);
    return ok;
}

int solock_config_get_timeout_minutes(SolockConfig *c) { return c->timeout_minutes; }
void solock_config_set_timeout_minutes(SolockConfig *c, int m) { c->timeout_minutes = m; }
int solock_config_get_clipboard_clear_seconds(SolockConfig *c) { return c->clipboard_clear_seconds; }
void solock_config_set_clipboard_clear_seconds(SolockConfig *c, int s) { c->clipboard_clear_seconds = s; }
const char *solock_config_get_paste_method(SolockConfig *c) { return c->paste_method; }
void solock_config_set_paste_method(SolockConfig *c, const char *m) { g_free(c->paste_method); c->paste_method = g_strdup(m); }

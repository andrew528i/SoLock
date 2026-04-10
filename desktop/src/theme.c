#include "solock-desktop.h"
#include <string.h>

static const char *theme_keys[] = {
    "bg",
    "bg_input",
    "fg",
    "fg_muted",
    "fg_dim",
    "fg_dimmer",
    "border",
    "accent",
    "success",
    "warning",
    "danger",
    "group_red",
    "group_orange",
    "group_yellow",
    "group_green",
    "group_teal",
    "group_blue",
    "group_purple",
    "group_pink",
    "group_gray",
    NULL,
};

static char *theme_config_path(void)
{
    const char *xdg = g_get_user_config_dir();
    return g_build_filename(xdg, "solock", "theme.ini", NULL);
}

static gboolean is_valid_color(const char *value)
{
    if (!value || !*value) return FALSE;
    if (value[0] != '#') return FALSE;
    gsize len = strlen(value);
    if (len != 4 && len != 7 && len != 9) return FALSE;
    for (gsize i = 1; i < len; i++) {
        char c = value[i];
        if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')))
            return FALSE;
    }
    return TRUE;
}

gboolean solock_theme_try_load(GtkCssProvider *provider)
{
    char *path = theme_config_path();
    if (!g_file_test(path, G_FILE_TEST_EXISTS)) {
        g_free(path);
        return FALSE;
    }

    GKeyFile *kf = g_key_file_new();
    GError *error = NULL;
    if (!g_key_file_load_from_file(kf, path, G_KEY_FILE_NONE, &error)) {
        g_warning("solock theme: failed to load %s: %s", path, error->message);
        g_error_free(error);
        g_key_file_free(kf);
        g_free(path);
        return FALSE;
    }

    GString *css = g_string_new(NULL);
    gboolean any = FALSE;

    for (const char **k = theme_keys; *k; k++) {
        gchar *value = g_key_file_get_string(kf, "colors", *k, NULL);
        if (!value) continue;
        gchar *trimmed = g_strstrip(value);
        if (!is_valid_color(trimmed)) {
            g_warning("solock theme: invalid color for %s: %s (skipped)", *k, trimmed);
            g_free(value);
            continue;
        }
        g_string_append_printf(css, "@define-color solock_%s %s;\n", *k, trimmed);
        g_free(value);
        any = TRUE;
    }

    g_key_file_free(kf);
    g_free(path);

    if (!any) {
        g_string_free(css, TRUE);
        return FALSE;
    }

    gtk_css_provider_load_from_string(provider, css->str);
    g_string_free(css, TRUE);
    return TRUE;
}

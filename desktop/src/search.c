#include "solock-desktop.h"
#include <string.h>

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);

static const char LABEL_CHARS[] = "asdfghjkl";
static const int  LABEL_CHARS_LEN = 9;

typedef struct {
    SolockApp  *app;
    GtkWidget  *search_revealer;
    GtkWidget  *search_entry;
    GtkWidget  *list_box;
    JsonNode   *entries;
    gboolean    label_mode;
} SearchData;

static void refresh_entries(SearchData *sd);

static const char *icon_for_type(const char *type)
{
    if (g_strcmp0(type, "password") == 0) return "dialog-password-symbolic";
    if (g_strcmp0(type, "card") == 0)     return "credit-card-symbolic";
    if (g_strcmp0(type, "note") == 0)     return "document-text-symbolic";
    if (g_strcmp0(type, "totp") == 0)     return "fingerprint-symbolic";
    return "dialog-password-symbolic";
}

static void update_label_hints(SearchData *sd)
{
    int idx = 0;
    for (GtkWidget *row = gtk_widget_get_first_child(sd->list_box);
         row != NULL;
         row = gtk_widget_get_next_sibling(row), idx++) {

        GtkWidget *row_content = gtk_list_box_row_get_child(GTK_LIST_BOX_ROW(row));
        if (!row_content) continue;

        GtkWidget *hint_revealer = gtk_widget_get_first_child(row_content);
        if (!hint_revealer || !GTK_IS_REVEALER(hint_revealer)) continue;

        gtk_revealer_set_reveal_child(GTK_REVEALER(hint_revealer), sd->label_mode);
    }
}

static char label_char_for_index(int idx)
{
    if (idx < LABEL_CHARS_LEN)
        return LABEL_CHARS[idx];
    return 0;
}

static int index_for_label_char(guint keyval)
{
    char ch = (char)gdk_keyval_to_unicode(keyval);
    if (ch == 0) return -1;

    for (int i = 0; i < LABEL_CHARS_LEN; i++) {
        if (LABEL_CHARS[i] == ch)
            return i;
    }
    return -1;
}

static void activate_entry_at_index(SearchData *sd, int idx)
{
    if (!sd->entries) return;

    JsonArray *arr = json_node_get_array(sd->entries);
    if (idx < 0 || (guint)idx >= json_array_get_length(arr)) return;

    JsonNode *entry = json_array_get_element(arr, idx);
    GtkWidget *popup = solock_app_get_popup(sd->app);
    solock_popup_switch_to_detail(popup, entry);
}

static void on_row_activated(GtkListBox *list_box, GtkListBoxRow *row, gpointer data)
{
    (void)list_box;
    SearchData *sd = data;
    int idx = gtk_list_box_row_get_index(row);
    activate_entry_at_index(sd, idx);
}

static void on_search_changed(GtkEditable *editable, gpointer data)
{
    (void)editable;
    SearchData *sd = data;
    refresh_entries(sd);
}

static void close_search(SearchData *sd)
{
    gtk_editable_set_text(GTK_EDITABLE(sd->search_entry), "");
    gtk_revealer_set_reveal_child(GTK_REVEALER(sd->search_revealer), FALSE);
    gtk_widget_grab_focus(sd->list_box);
    refresh_entries(sd);
}

static gboolean on_key_pressed(GtkEventControllerKey *ctrl, guint keyval,
                                guint keycode, GdkModifierType state, gpointer data)
{
    (void)ctrl; (void)keycode;
    SearchData *sd = data;

    if (keyval == GDK_KEY_Control_L || keyval == GDK_KEY_Control_R) {
        sd->label_mode = TRUE;
        update_label_hints(sd);
        return TRUE;
    }

    if (sd->label_mode && !(state & GDK_CONTROL_MASK)) {
        sd->label_mode = FALSE;
        update_label_hints(sd);
    }

    if (state & GDK_CONTROL_MASK) {
        int idx = index_for_label_char(keyval);
        if (idx >= 0) {
            activate_entry_at_index(sd, idx);
            return TRUE;
        }
    }

    if (keyval == GDK_KEY_Escape) {
        if (gtk_revealer_get_reveal_child(GTK_REVEALER(sd->search_revealer))) {
            close_search(sd);
            return TRUE;
        }
        GtkWidget *popup = solock_app_get_popup(sd->app);
        solock_popup_hide(popup);
        return TRUE;
    }

    if (keyval == GDK_KEY_Return || keyval == GDK_KEY_KP_Enter) {
        GtkListBoxRow *row = gtk_list_box_get_selected_row(GTK_LIST_BOX(sd->list_box));
        if (row) {
            int idx = gtk_list_box_row_get_index(row);
            activate_entry_at_index(sd, idx);
            return TRUE;
        }
        return FALSE;
    }

    if (keyval == GDK_KEY_Up || keyval == GDK_KEY_Down) {
        GtkListBoxRow *selected = gtk_list_box_get_selected_row(GTK_LIST_BOX(sd->list_box));
        if (!selected) {
            GtkListBoxRow *first = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sd->list_box), 0);
            if (first)
                gtk_list_box_select_row(GTK_LIST_BOX(sd->list_box), first);
            return TRUE;
        }

        int cur = gtk_list_box_row_get_index(selected);
        int next = (keyval == GDK_KEY_Down) ? cur + 1 : cur - 1;
        GtkListBoxRow *next_row = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sd->list_box), next);
        if (next_row)
            gtk_list_box_select_row(GTK_LIST_BOX(sd->list_box), next_row);
        return TRUE;
    }

    if (keyval == GDK_KEY_BackSpace) {
        if (gtk_revealer_get_reveal_child(GTK_REVEALER(sd->search_revealer))) {
            const char *text = gtk_editable_get_text(GTK_EDITABLE(sd->search_entry));
            if (text && *text) {
                gtk_widget_grab_focus(sd->search_entry);
                return FALSE;
            }
            close_search(sd);
            return TRUE;
        }
        return FALSE;
    }

    guint32 ch = gdk_keyval_to_unicode(keyval);
    if (ch > 0 && g_unichar_isprint(ch) && !(state & GDK_CONTROL_MASK) && !(state & GDK_ALT_MASK)) {
        if (!gtk_revealer_get_reveal_child(GTK_REVEALER(sd->search_revealer))) {
            gtk_revealer_set_reveal_child(GTK_REVEALER(sd->search_revealer), TRUE);
        }
        gtk_widget_grab_focus(sd->search_entry);
        char buf[8];
        int len = g_unichar_to_utf8(ch, buf);
        buf[len] = '\0';
        int pos = gtk_editable_get_position(GTK_EDITABLE(sd->search_entry));
        gtk_editable_insert_text(GTK_EDITABLE(sd->search_entry), buf, len, &pos);
        gtk_editable_set_position(GTK_EDITABLE(sd->search_entry), pos);
        return TRUE;
    }

    return FALSE;
}

static void on_key_released(GtkEventControllerKey *ctrl, guint keyval,
                             guint keycode, GdkModifierType state, gpointer data)
{
    (void)ctrl; (void)keycode; (void)state;
    SearchData *sd = data;

    if (keyval == GDK_KEY_Control_L || keyval == GDK_KEY_Control_R) {
        sd->label_mode = FALSE;
        update_label_hints(sd);
    }
}

static void refresh_entries(SearchData *sd)
{
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(sd->list_box)) != NULL)
        gtk_list_box_remove(GTK_LIST_BOX(sd->list_box), child);

    if (sd->entries) {
        json_node_unref(sd->entries);
        sd->entries = NULL;
    }

    SolockClient *client = solock_app_get_client(sd->app);
    GError *error = NULL;
    const char *query = gtk_editable_get_text(GTK_EDITABLE(sd->search_entry));

    if (query && *query)
        sd->entries = solock_client_search_entries(client, query, &error);
    else
        sd->entries = solock_client_list_entries(client, &error);

    if (error) {
        g_warning("Failed to fetch entries: %s", error->message);
        g_error_free(error);
    }

    if (!sd->entries) return;

    if (JSON_NODE_TYPE(sd->entries) != JSON_NODE_ARRAY) return;

    JsonArray *arr = json_node_get_array(sd->entries);
    guint len = json_array_get_length(arr);

    for (guint i = 0; i < len; i++) {
        JsonObject *obj = json_array_get_object_element(arr, i);
        const char *name = json_object_get_string_member(obj, "name");
        const char *type = json_object_get_string_member(obj, "type");
        gboolean has_totp = json_object_get_boolean_member_with_default(obj, "has_totp", FALSE);

        JsonObject *fields = json_object_get_object_member(obj, "fields");
        const char *subtitle = NULL;
        if (fields) {
            if (json_object_has_member(fields, "site"))
                subtitle = json_object_get_string_member(fields, "site");
            else if (json_object_has_member(fields, "username"))
                subtitle = json_object_get_string_member(fields, "username");
            else if (json_object_has_member(fields, "cardholder"))
                subtitle = json_object_get_string_member(fields, "cardholder");
        }

        GtkWidget *row_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 12);
        gtk_widget_add_css_class(row_box, "entry-row");
        gtk_widget_set_margin_start(row_box, 10);
        gtk_widget_set_margin_end(row_box, 10);
        gtk_widget_set_margin_top(row_box, 6);
        gtk_widget_set_margin_bottom(row_box, 6);

        char lbl_char = label_char_for_index(i);
        GtkWidget *hint_revealer = gtk_revealer_new();
        gtk_revealer_set_transition_type(GTK_REVEALER(hint_revealer), GTK_REVEALER_TRANSITION_TYPE_CROSSFADE);
        gtk_revealer_set_transition_duration(GTK_REVEALER(hint_revealer), 100);
        gtk_revealer_set_reveal_child(GTK_REVEALER(hint_revealer), FALSE);
        if (lbl_char) {
            char lbl_str[2] = { lbl_char, '\0' };
            GtkWidget *hint = gtk_label_new(lbl_str);
            gtk_widget_add_css_class(hint, "label-hint");
            gtk_revealer_set_child(GTK_REVEALER(hint_revealer), hint);
        }
        gtk_box_append(GTK_BOX(row_box), hint_revealer);

        GtkWidget *icon = gtk_image_new_from_icon_name(icon_for_type(type));
        gtk_image_set_pixel_size(GTK_IMAGE(icon), 18);
        gtk_widget_add_css_class(icon, "entry-icon");
        gtk_box_append(GTK_BOX(row_box), icon);

        GtkWidget *text_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
        gtk_widget_set_hexpand(text_box, TRUE);
        gboolean has_subtitle = subtitle && *subtitle;
        if (!has_subtitle)
            gtk_widget_set_valign(text_box, GTK_ALIGN_CENTER);

        GtkWidget *name_label = gtk_label_new(name);
        gtk_widget_add_css_class(name_label, "entry-name");
        gtk_label_set_xalign(GTK_LABEL(name_label), 0);
        gtk_label_set_ellipsize(GTK_LABEL(name_label), PANGO_ELLIPSIZE_END);
        gtk_box_append(GTK_BOX(text_box), name_label);

        if (has_subtitle) {
            GtkWidget *sub_label = gtk_label_new(subtitle);
            gtk_widget_add_css_class(sub_label, "entry-subtitle");
            gtk_label_set_xalign(GTK_LABEL(sub_label), 0);
            gtk_label_set_ellipsize(GTK_LABEL(sub_label), PANGO_ELLIPSIZE_END);
            gtk_box_append(GTK_BOX(text_box), sub_label);
        }

        gtk_box_append(GTK_BOX(row_box), text_box);

        if (has_totp) {
            GtkWidget *totp_indicator = gtk_label_new("\xe2\x97\x8f");
            gtk_widget_add_css_class(totp_indicator, "totp-indicator");
            gtk_widget_set_valign(totp_indicator, GTK_ALIGN_CENTER);
            gtk_box_append(GTK_BOX(row_box), totp_indicator);
        }

        gtk_list_box_append(GTK_LIST_BOX(sd->list_box), row_box);
    }

    GtkListBoxRow *first = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sd->list_box), 0);
    if (first)
        gtk_list_box_select_row(GTK_LIST_BOX(sd->list_box), first);
}

static void on_view_map(GtkWidget *widget, gpointer data)
{
    (void)widget;
    SearchData *sd = data;
    sd->label_mode = FALSE;
    gtk_editable_set_text(GTK_EDITABLE(sd->search_entry), "");
    gtk_revealer_set_reveal_child(GTK_REVEALER(sd->search_revealer), FALSE);
    refresh_entries(sd);
    gtk_widget_grab_focus(sd->list_box);
}

GtkWidget *solock_search_view_new(SolockApp *app)
{
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);

    GtkWidget *search_revealer = gtk_revealer_new();
    gtk_revealer_set_transition_type(GTK_REVEALER(search_revealer), GTK_REVEALER_TRANSITION_TYPE_SLIDE_DOWN);
    gtk_revealer_set_transition_duration(GTK_REVEALER(search_revealer), 150);
    gtk_revealer_set_reveal_child(GTK_REVEALER(search_revealer), FALSE);

    GtkWidget *search_entry = gtk_search_entry_new();
    gtk_widget_add_css_class(search_entry, "search-input");
    gtk_widget_set_margin_start(search_entry, 10);
    gtk_widget_set_margin_end(search_entry, 10);
    gtk_widget_set_margin_top(search_entry, 8);
    gtk_widget_set_margin_bottom(search_entry, 4);
    gtk_revealer_set_child(GTK_REVEALER(search_revealer), search_entry);
    gtk_box_append(GTK_BOX(box), search_revealer);

    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
    gtk_scrolled_window_set_max_content_height(GTK_SCROLLED_WINDOW(scroll), 380);
    gtk_scrolled_window_set_propagate_natural_height(GTK_SCROLLED_WINDOW(scroll), TRUE);

    GtkWidget *list_box = gtk_list_box_new();
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(list_box), GTK_SELECTION_SINGLE);
    gtk_widget_add_css_class(list_box, "entry-list");
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), list_box);
    gtk_box_append(GTK_BOX(box), scroll);

    SearchData *sd = g_new0(SearchData, 1);
    sd->app = app;
    sd->search_revealer = search_revealer;
    sd->search_entry = search_entry;
    sd->list_box = list_box;
    sd->entries = NULL;
    sd->label_mode = FALSE;

    g_signal_connect(search_entry, "search-changed", G_CALLBACK(on_search_changed), sd);
    g_signal_connect(list_box, "row-activated", G_CALLBACK(on_row_activated), sd);
    g_signal_connect(box, "map", G_CALLBACK(on_view_map), sd);

    GtkEventController *key_ctrl = gtk_event_controller_key_new();
    gtk_event_controller_set_propagation_phase(key_ctrl, GTK_PHASE_CAPTURE);
    g_signal_connect(key_ctrl, "key-pressed", G_CALLBACK(on_key_pressed), sd);
    g_signal_connect(key_ctrl, "key-released", G_CALLBACK(on_key_released), sd);
    gtk_widget_add_controller(box, key_ctrl);

    return box;
}

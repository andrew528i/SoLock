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
    GtkWidget  *group_bar;
    GtkWidget  *group_scroll;
    GtkWidget  *list_box;
    JsonNode   *entries;
    JsonNode   *groups;
    int         active_group; /* -1 = all */
    gboolean    label_mode;
} SearchData;

static void refresh_entries(SearchData *sd);
static void rebuild_group_bar(SearchData *sd);

static void on_group_chip_clicked(GtkButton *btn, gpointer data)
{
    SearchData *sd = data;
    sd->active_group = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(btn), "group-idx"));
    rebuild_group_bar(sd);
    refresh_entries(sd);
}

static void rebuild_group_bar(SearchData *sd)
{
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(sd->group_bar)))
        gtk_box_remove(GTK_BOX(sd->group_bar), child);

    if (sd->groups)
        json_node_unref(sd->groups);
    sd->groups = NULL;

    SolockClient *client = solock_app_get_client(sd->app);
    sd->groups = solock_client_list_groups(client, NULL);

    int active_count = 0;
    if (sd->groups && JSON_NODE_TYPE(sd->groups) == JSON_NODE_ARRAY) {
        JsonArray *arr = json_node_get_array(sd->groups);
        for (guint i = 0; i < json_array_get_length(arr); i++) {
            JsonObject *obj = json_array_get_object_element(arr, i);
            if (!json_object_get_boolean_member(obj, "deleted"))
                active_count++;
        }
    }

    if (active_count == 0) {
        gtk_widget_set_visible(sd->group_scroll, FALSE);
        sd->active_group = -1;
        return;
    }

    gtk_widget_set_visible(sd->group_scroll, TRUE);

    GtkWidget *all_btn = gtk_button_new_with_label("All");
    gtk_widget_add_css_class(all_btn, "flat");
    gtk_widget_add_css_class(all_btn, "group-chip");
    if (sd->active_group == -1)
        gtk_widget_add_css_class(all_btn, "group-chip-active");
    g_object_set_data(G_OBJECT(all_btn), "group-idx", GINT_TO_POINTER(-1));
    g_signal_connect(all_btn, "clicked", G_CALLBACK(on_group_chip_clicked), sd);
    gtk_box_append(GTK_BOX(sd->group_bar), all_btn);

    JsonArray *arr = json_node_get_array(sd->groups);
    for (guint i = 0; i < json_array_get_length(arr); i++) {
        JsonObject *obj = json_array_get_object_element(arr, i);
        if (json_object_get_boolean_member(obj, "deleted"))
            continue;

        const char *name = json_object_get_string_member(obj, "name");
        int index = (int)json_object_get_int_member(obj, "index");

        GtkWidget *btn = gtk_button_new_with_label(name);
        gtk_widget_add_css_class(btn, "flat");
        gtk_widget_add_css_class(btn, "group-chip");
        gtk_label_set_ellipsize(
            GTK_LABEL(gtk_button_get_child(GTK_BUTTON(btn))),
            PANGO_ELLIPSIZE_END);
        gtk_label_set_max_width_chars(
            GTK_LABEL(gtk_button_get_child(GTK_BUTTON(btn))), 16);
        if (sd->active_group == index)
            gtk_widget_add_css_class(btn, "group-chip-active");
        g_object_set_data(G_OBJECT(btn), "group-idx", GINT_TO_POINTER(index));
        g_signal_connect(btn, "clicked", G_CALLBACK(on_group_chip_clicked), sd);
        gtk_box_append(GTK_BOX(sd->group_bar), btn);
    }
}

static void cycle_group_forward(SearchData *sd)
{
    if (!sd->groups || JSON_NODE_TYPE(sd->groups) != JSON_NODE_ARRAY)
        return;

    JsonArray *arr = json_node_get_array(sd->groups);
    guint len = json_array_get_length(arr);
    if (len == 0) return;

    if (sd->active_group == -1) {
        for (guint i = 0; i < len; i++) {
            JsonObject *obj = json_array_get_object_element(arr, i);
            if (!json_object_get_boolean_member(obj, "deleted")) {
                sd->active_group = (int)json_object_get_int_member(obj, "index");
                break;
            }
        }
    } else {
        gboolean found = FALSE;
        gboolean pick_next = FALSE;
        for (guint i = 0; i < len; i++) {
            JsonObject *obj = json_array_get_object_element(arr, i);
            if (json_object_get_boolean_member(obj, "deleted"))
                continue;
            int idx = (int)json_object_get_int_member(obj, "index");
            if (pick_next) {
                sd->active_group = idx;
                found = TRUE;
                break;
            }
            if (idx == sd->active_group)
                pick_next = TRUE;
        }
        if (!found)
            sd->active_group = -1;
    }

    rebuild_group_bar(sd);
    refresh_entries(sd);
}

static void cycle_group_backward(SearchData *sd)
{
    if (!sd->groups || JSON_NODE_TYPE(sd->groups) != JSON_NODE_ARRAY)
        return;

    JsonArray *arr = json_node_get_array(sd->groups);
    guint len = json_array_get_length(arr);
    if (len == 0) return;

    if (sd->active_group == -1) {
        for (int i = (int)len - 1; i >= 0; i--) {
            JsonObject *obj = json_array_get_object_element(arr, (guint)i);
            if (!json_object_get_boolean_member(obj, "deleted")) {
                sd->active_group = (int)json_object_get_int_member(obj, "index");
                break;
            }
        }
    } else {
        int prev = -1;
        for (guint i = 0; i < len; i++) {
            JsonObject *obj = json_array_get_object_element(arr, i);
            if (json_object_get_boolean_member(obj, "deleted"))
                continue;
            int idx = (int)json_object_get_int_member(obj, "index");
            if (idx == sd->active_group)
                break;
            prev = idx;
        }
        sd->active_group = prev;
    }

    rebuild_group_bar(sd);
    refresh_entries(sd);
}

static const char *icon_for_type(const char *type)
{
    if (g_strcmp0(type, "password") == 0) return "dialog-password-symbolic";
    if (g_strcmp0(type, "card") == 0)     return "credit-card-symbolic";
    if (g_strcmp0(type, "note") == 0)     return "accessories-text-editor-symbolic";
    if (g_strcmp0(type, "totp") == 0)     return "fingerprint-symbolic";
    return "dialog-password-symbolic";
}

static void update_label_hints(SearchData *sd)
{
    for (GtkWidget *row = gtk_widget_get_first_child(sd->list_box);
         row != NULL;
         row = gtk_widget_get_next_sibling(row)) {

        GtkWidget *row_content = gtk_list_box_row_get_child(GTK_LIST_BOX_ROW(row));
        if (!row_content) continue;

        for (GtkWidget *child = gtk_widget_get_first_child(row_content);
             child != NULL;
             child = gtk_widget_get_next_sibling(child)) {
            if (GTK_IS_REVEALER(child)) {
                gtk_revealer_set_reveal_child(GTK_REVEALER(child), sd->label_mode);
                break;
            }
        }
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
    gtk_widget_add_css_class(sd->list_box, "keyboard-nav");
    refresh_entries(sd);
}

static void close_search(SearchData *sd)
{
    gtk_editable_set_text(GTK_EDITABLE(sd->search_entry), "");
    gtk_revealer_set_reveal_child(GTK_REVEALER(sd->search_revealer), FALSE);
    gtk_widget_grab_focus(sd->list_box);
    refresh_entries(sd);
}

static void on_list_mouse_motion(GtkEventControllerMotion *ctrl,
                                 double x, double y, gpointer data)
{
    (void)ctrl; (void)x; (void)y;
    SearchData *sd = data;
    gtk_widget_remove_css_class(sd->list_box, "keyboard-nav");
}

static gboolean on_key_pressed(GtkEventControllerKey *ctrl, guint keyval,
                                guint keycode, GdkModifierType state, gpointer data)
{
    (void)ctrl; (void)keycode;
    SearchData *sd = data;

    gtk_widget_add_css_class(sd->list_box, "keyboard-nav");

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

    if (keyval == GDK_KEY_Tab && !(state & GDK_SHIFT_MASK)) {
        cycle_group_forward(sd);
        return TRUE;
    }
    if ((keyval == GDK_KEY_Tab && (state & GDK_SHIFT_MASK)) ||
        keyval == GDK_KEY_ISO_Left_Tab) {
        cycle_group_backward(sd);
        return TRUE;
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
            gtk_widget_grab_focus(sd->list_box);
            return TRUE;
        }

        int cur = gtk_list_box_row_get_index(selected);
        int next = (keyval == GDK_KEY_Down) ? cur + 1 : cur - 1;
        GtkListBoxRow *next_row = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sd->list_box), next);
        if (next_row)
            gtk_list_box_select_row(GTK_LIST_BOX(sd->list_box), next_row);
        gtk_widget_grab_focus(sd->list_box);
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

    if (!sd->entries || JSON_NODE_TYPE(sd->entries) != JSON_NODE_ARRAY) {
        if (gtk_revealer_get_reveal_child(GTK_REVEALER(sd->search_revealer))) {
            GtkWidget *no_results = gtk_label_new("No results");
            gtk_widget_add_css_class(no_results, "dim-label");
            gtk_widget_set_margin_top(no_results, 8);
            gtk_widget_set_margin_bottom(no_results, 8);
            gtk_list_box_append(GTK_LIST_BOX(sd->list_box), no_results);
        } else {
            GtkWidget *empty_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 4);
            gtk_widget_set_halign(empty_box, GTK_ALIGN_CENTER);
            gtk_widget_set_valign(empty_box, GTK_ALIGN_CENTER);
            gtk_widget_set_margin_top(empty_box, 24);
            gtk_widget_set_margin_bottom(empty_box, 24);

            GtkWidget *empty_title = gtk_label_new("No entries yet");
            gtk_widget_add_css_class(empty_title, "dim-label");
            gtk_box_append(GTK_BOX(empty_box), empty_title);

            GtkWidget *empty_sub = gtk_label_new("Add passwords via Manage Vault");
            gtk_widget_add_css_class(empty_sub, "dim-label");
            gtk_widget_add_css_class(empty_sub, "caption");
            gtk_box_append(GTK_BOX(empty_box), empty_sub);

            gtk_list_box_append(GTK_LIST_BOX(sd->list_box), empty_box);
        }
        return;
    }

    JsonArray *arr = json_node_get_array(sd->entries);
    guint len = json_array_get_length(arr);

    guint visible_count = 0;
    for (guint i = 0; i < len; i++) {
        JsonObject *obj = json_array_get_object_element(arr, i);

        if (sd->active_group >= 0) {
            if (!json_object_has_member(obj, "group_index") ||
                json_object_get_null_member(obj, "group_index") ||
                (int)json_object_get_int_member(obj, "group_index") != sd->active_group) {
                continue;
            }
        }
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

        GtkWidget *row_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
        gtk_widget_add_css_class(row_box, "entry-row");
        gtk_widget_set_margin_start(row_box, 4);
        gtk_widget_set_margin_end(row_box, 4);
        gtk_widget_set_margin_top(row_box, 4);
        gtk_widget_set_margin_bottom(row_box, 4);
        gtk_widget_set_size_request(row_box, -1, 48);

        char lbl_char = label_char_for_index(i);
        GtkWidget *hint_revealer = gtk_revealer_new();
        gtk_revealer_set_transition_type(GTK_REVEALER(hint_revealer), GTK_REVEALER_TRANSITION_TYPE_SLIDE_RIGHT);
        gtk_revealer_set_transition_duration(GTK_REVEALER(hint_revealer), 100);
        gtk_revealer_set_reveal_child(GTK_REVEALER(hint_revealer), FALSE);
        gtk_widget_set_margin_start(hint_revealer, 4);
        if (lbl_char) {
            char lbl_str[2] = { lbl_char, '\0' };
            GtkWidget *hint = gtk_label_new(lbl_str);
            gtk_widget_add_css_class(hint, "label-hint");
            gtk_widget_add_css_class(hint, "label-hint-visible");
            gtk_widget_set_margin_end(hint, 4);
            gtk_revealer_set_child(GTK_REVEALER(hint_revealer), hint);
        }
        gtk_box_append(GTK_BOX(row_box), hint_revealer);

        GtkWidget *icon = gtk_image_new_from_icon_name(icon_for_type(type));
        gtk_image_set_pixel_size(GTK_IMAGE(icon), 18);
        gtk_widget_add_css_class(icon, "entry-icon");
        gtk_widget_set_valign(icon, GTK_ALIGN_CENTER);
        gtk_widget_set_margin_start(icon, 11);
        gtk_widget_set_margin_end(icon, 10);
        gtk_box_append(GTK_BOX(row_box), icon);

        GtkWidget *text_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
        gtk_widget_set_hexpand(text_box, TRUE);
        gtk_widget_set_valign(text_box, GTK_ALIGN_CENTER);
        gboolean has_subtitle = subtitle && *subtitle;

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
            gtk_widget_set_margin_end(totp_indicator, 10);
            gtk_box_append(GTK_BOX(row_box), totp_indicator);
        }

        gtk_list_box_append(GTK_LIST_BOX(sd->list_box), row_box);
        visible_count++;
    }

    if (visible_count == 0) {
        if (gtk_revealer_get_reveal_child(GTK_REVEALER(sd->search_revealer))) {
            GtkWidget *no_results = gtk_label_new("No results");
            gtk_widget_add_css_class(no_results, "dim-label");
            gtk_widget_set_margin_top(no_results, 8);
            gtk_widget_set_margin_bottom(no_results, 8);
            gtk_list_box_append(GTK_LIST_BOX(sd->list_box), no_results);
        } else {
            GtkWidget *empty_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 4);
            gtk_widget_set_halign(empty_box, GTK_ALIGN_CENTER);
            gtk_widget_set_valign(empty_box, GTK_ALIGN_CENTER);
            gtk_widget_set_margin_top(empty_box, 24);
            gtk_widget_set_margin_bottom(empty_box, 24);

            GtkWidget *empty_title = gtk_label_new("No entries yet");
            gtk_widget_add_css_class(empty_title, "dim-label");
            gtk_box_append(GTK_BOX(empty_box), empty_title);

            GtkWidget *empty_sub = gtk_label_new("Add passwords via Manage Vault");
            gtk_widget_add_css_class(empty_sub, "dim-label");
            gtk_widget_add_css_class(empty_sub, "caption");
            gtk_box_append(GTK_BOX(empty_box), empty_sub);

            gtk_list_box_append(GTK_LIST_BOX(sd->list_box), empty_box);
        }
        return;
    }

    GtkListBoxRow *first = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sd->list_box), 0);
    if (first)
        gtk_list_box_select_row(GTK_LIST_BOX(sd->list_box), first);

    gtk_widget_queue_draw(sd->list_box);
}

static void on_view_map(GtkWidget *widget, gpointer data)
{
    (void)widget;
    SearchData *sd = data;
    sd->label_mode = FALSE;
    gtk_editable_set_text(GTK_EDITABLE(sd->search_entry), "");
    gtk_revealer_set_reveal_child(GTK_REVEALER(sd->search_revealer), FALSE);
    rebuild_group_bar(sd);
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
    gtk_widget_set_margin_start(search_entry, 6);
    gtk_widget_set_margin_end(search_entry, 6);
    gtk_widget_set_margin_top(search_entry, 6);
    gtk_widget_set_margin_bottom(search_entry, 2);
    gtk_revealer_set_child(GTK_REVEALER(search_revealer), search_entry);
    gtk_box_append(GTK_BOX(box), search_revealer);

    GtkWidget *group_scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(group_scroll),
                                   GTK_POLICY_AUTOMATIC, GTK_POLICY_NEVER);
    gtk_widget_set_size_request(group_scroll, -1, -1);

    GtkWidget *group_bar = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 4);
    gtk_widget_set_margin_start(group_bar, 8);
    gtk_widget_set_margin_end(group_bar, 8);
    gtk_widget_set_margin_top(group_bar, 4);
    gtk_widget_set_margin_bottom(group_bar, 2);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(group_scroll), group_bar);
    gtk_widget_set_visible(group_scroll, FALSE);
    gtk_box_append(GTK_BOX(box), group_scroll);

    GtkWidget *scroll = gtk_scrolled_window_new();
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
    gtk_scrolled_window_set_max_content_height(GTK_SCROLLED_WINDOW(scroll), 320);
    gtk_scrolled_window_set_propagate_natural_height(GTK_SCROLLED_WINDOW(scroll), TRUE);
    gtk_widget_set_vexpand(scroll, FALSE);
    gtk_widget_set_size_request(scroll, 280, 0);

    GtkWidget *list_box = gtk_list_box_new();
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(list_box), GTK_SELECTION_SINGLE);
    gtk_widget_add_css_class(list_box, "entry-list");
    gtk_widget_set_margin_top(list_box, 6);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(scroll), list_box);
    gtk_box_append(GTK_BOX(box), scroll);

    SearchData *sd = g_new0(SearchData, 1);
    sd->app = app;
    sd->search_revealer = search_revealer;
    sd->search_entry = search_entry;
    sd->group_bar = group_bar;
    sd->group_scroll = group_scroll;
    sd->list_box = list_box;
    sd->entries = NULL;
    sd->groups = NULL;
    sd->active_group = -1;
    sd->label_mode = FALSE;

    g_signal_connect(search_entry, "search-changed", G_CALLBACK(on_search_changed), sd);
    g_signal_connect(list_box, "row-activated", G_CALLBACK(on_row_activated), sd);
    g_signal_connect(box, "map", G_CALLBACK(on_view_map), sd);

    GtkEventController *key_ctrl = gtk_event_controller_key_new();
    gtk_event_controller_set_propagation_phase(key_ctrl, GTK_PHASE_CAPTURE);
    g_signal_connect(key_ctrl, "key-pressed", G_CALLBACK(on_key_pressed), sd);
    g_signal_connect(key_ctrl, "key-released", G_CALLBACK(on_key_released), sd);
    gtk_widget_add_controller(box, key_ctrl);

    GtkEventController *mouse_ctrl = gtk_event_controller_motion_new();
    g_signal_connect(mouse_ctrl, "motion", G_CALLBACK(on_list_mouse_motion), sd);
    gtk_widget_add_controller(list_box, mouse_ctrl);

    return box;
}

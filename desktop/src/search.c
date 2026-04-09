#include "solock-desktop.h"
#include <string.h>

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);

static const char LABEL_CHARS[] = "asdfghjkl;";
static const int  LABEL_CHARS_LEN = 10;
#define MAX_VISIBLE_ENTRIES 6

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
    int        *filtered_indices;
    int         filtered_count;
    int         visible_offset;
    int         selected_filtered;
} SearchData;

static void refresh_entries(SearchData *sd);
static void rebuild_visible_list(SearchData *sd);
static void rebuild_group_bar(SearchData *sd);

static gboolean do_scroll_to_active_chip(gpointer data)
{
    SearchData *sd = data;
    GtkAdjustment *adj = gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(sd->group_scroll));
    for (GtkWidget *child = gtk_widget_get_first_child(sd->group_bar);
         child; child = gtk_widget_get_next_sibling(child)) {
        if (gtk_widget_has_css_class(child, "group-chip-active")) {
            graphene_rect_t bounds;
            if (gtk_widget_compute_bounds(child, sd->group_bar, &bounds)) {
                double mid = bounds.origin.x + bounds.size.width / 2.0;
                double page = gtk_adjustment_get_page_size(adj);
                double target = mid - page / 2.0;
                double upper = gtk_adjustment_get_upper(adj);
                if (target < 0) target = 0;
                if (target > upper - page) target = upper - page;
                gtk_adjustment_set_value(adj, target);
            }
            break;
        }
    }
    return G_SOURCE_REMOVE;
}

static void scroll_to_active_chip(SearchData *sd)
{
    g_idle_add(do_scroll_to_active_chip, sd);
    g_timeout_add(50, do_scroll_to_active_chip, sd);
}

static void on_group_chip_clicked(GtkButton *btn, gpointer data)
{
    SearchData *sd = data;
    sd->active_group = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(btn), "group-idx"));
    rebuild_group_bar(sd);
    refresh_entries(sd);
    scroll_to_active_chip(sd);
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
        const char *color = NULL;
        if (json_object_has_member(obj, "color") && !json_object_get_null_member(obj, "color"))
            color = json_object_get_string_member(obj, "color");

        GtkWidget *btn = gtk_button_new();
        GtkWidget *btn_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 4);
        if (color && *color) {
            GtkWidget *chip_dot = gtk_label_new("\xe2\x97\x8f");
            gtk_widget_add_css_class(chip_dot, "group-chip-dot");
            char css_class[32];
            g_snprintf(css_class, sizeof(css_class), "gc-%s", color);
            gtk_widget_add_css_class(chip_dot, css_class);
            gtk_box_append(GTK_BOX(btn_box), chip_dot);
        }
        GtkWidget *chip_label = gtk_label_new(name);
        gtk_box_append(GTK_BOX(btn_box), chip_label);
        gtk_button_set_child(GTK_BUTTON(btn), btn_box);

        gtk_widget_add_css_class(btn, "flat");
        gtk_widget_add_css_class(btn, "group-chip");
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
    scroll_to_active_chip(sd);
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
    scroll_to_active_chip(sd);
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
    GtkWidget *child = gtk_list_box_row_get_child(row);
    if (!child || !gtk_widget_has_css_class(child, "entry-row")) return;
    int idx = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(child), "entry-index"));
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

static void on_list_mouse_motion(GtkEventControllerMotion *ctrl,
                                 double x, double y, gpointer data)
{
    (void)ctrl; (void)x; (void)y; (void)data;
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
        int label_idx = index_for_label_char(keyval);
        if (label_idx >= 0) {
            int row_idx = label_idx + (sd->visible_offset > 0 ? 1 : 0);
            GtkListBoxRow *row = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sd->list_box), row_idx);
            if (row) {
                GtkWidget *child = gtk_list_box_row_get_child(row);
                if (child && gtk_widget_has_css_class(child, "entry-row")) {
                    int entry_idx = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(child), "entry-index"));
                    activate_entry_at_index(sd, entry_idx);
                }
            }
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
            GtkWidget *child = gtk_list_box_row_get_child(row);
            if (child && gtk_widget_has_css_class(child, "entry-row")) {
                int idx = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(child), "entry-index"));
                activate_entry_at_index(sd, idx);
            }
            return TRUE;
        }
        return FALSE;
    }

    if (keyval == GDK_KEY_Up || keyval == GDK_KEY_Down) {
        if (sd->filtered_count == 0) return TRUE;
        int next_sel = sd->selected_filtered + (keyval == GDK_KEY_Down ? 1 : -1);
        if (next_sel < 0 || next_sel >= sd->filtered_count) return TRUE;
        sd->selected_filtered = next_sel;
        if (next_sel < sd->visible_offset)
            sd->visible_offset = next_sel;
        else if (next_sel >= sd->visible_offset + MAX_VISIBLE_ENTRIES)
            sd->visible_offset = next_sel - MAX_VISIBLE_ENTRIES + 1;
        rebuild_visible_list(sd);
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

static const char *group_color_for_index(SearchData *sd, int group_idx)
{
    if (group_idx < 0 || !sd->groups || JSON_NODE_TYPE(sd->groups) != JSON_NODE_ARRAY)
        return NULL;
    JsonArray *garr = json_node_get_array(sd->groups);
    for (guint i = 0; i < json_array_get_length(garr); i++) {
        JsonObject *gobj = json_array_get_object_element(garr, i);
        if ((int)json_object_get_int_member(gobj, "index") == group_idx) {
            if (json_object_has_member(gobj, "color") && !json_object_get_null_member(gobj, "color")) {
                const char *c = json_object_get_string_member(gobj, "color");
                return (c && *c) ? c : NULL;
            }
            return NULL;
        }
    }
    return NULL;
}

static int sort_compare_recently_used(gconstpointer a, gconstpointer b)
{
    JsonObject *oa = json_node_get_object(*(JsonNode **)a);
    JsonObject *ob = json_node_get_object(*(JsonNode **)b);
    gint64 aa = json_object_get_int_member_with_default(oa, "accessed_at", 0);
    gint64 ab = json_object_get_int_member_with_default(ob, "accessed_at", 0);
    if (aa != ab)
        return (ab > aa) ? 1 : -1;
    gint64 sa = json_object_get_int_member_with_default(oa, "slot_index", 0);
    gint64 sb = json_object_get_int_member_with_default(ob, "slot_index", 0);
    if (sa > sb) return 1;
    if (sa < sb) return -1;
    return 0;
}

static void sort_entries_by_recent(JsonArray *arr)
{
    guint len = json_array_get_length(arr);
    if (len <= 1) return;

    JsonNode **nodes = g_new(JsonNode *, len);
    for (guint i = 0; i < len; i++)
        nodes[i] = json_array_get_element(arr, i);

    qsort(nodes, len, sizeof(JsonNode *), sort_compare_recently_used);

    for (guint i = 0; i < len; i++)
        json_node_ref(nodes[i]);

    while (json_array_get_length(arr) > 0)
        json_array_remove_element(arr, 0);

    for (guint i = 0; i < len; i++)
        json_array_add_element(arr, nodes[i]);

    g_free(nodes);
}

static void refresh_entries(SearchData *sd)
{
    g_free(sd->filtered_indices);
    sd->filtered_indices = NULL;
    sd->filtered_count = 0;
    sd->visible_offset = 0;
    sd->selected_filtered = 0;

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
        if (solock_client_is_locked(client))
            solock_tray_update_status(sd->app, TRUE);
        g_error_free(error);
    }

    if (!sd->entries || JSON_NODE_TYPE(sd->entries) != JSON_NODE_ARRAY) {
        rebuild_visible_list(sd);
        return;
    }

    JsonArray *arr = json_node_get_array(sd->entries);
    sort_entries_by_recent(arr);
    guint len = json_array_get_length(arr);

    sd->filtered_indices = g_new(int, len);
    sd->filtered_count = 0;

    for (guint i = 0; i < len; i++) {
        JsonObject *obj = json_array_get_object_element(arr, i);
        if (sd->active_group >= 0) {
            if (!json_object_has_member(obj, "group_index") ||
                json_object_get_null_member(obj, "group_index") ||
                (int)json_object_get_int_member(obj, "group_index") != sd->active_group)
                continue;
        }
        sd->filtered_indices[sd->filtered_count++] = (int)i;
    }

    rebuild_visible_list(sd);
}

static void rebuild_visible_list(SearchData *sd)
{
    GtkWidget *child;
    while ((child = gtk_widget_get_first_child(sd->list_box)) != NULL)
        gtk_list_box_remove(GTK_LIST_BOX(sd->list_box), child);

    if (sd->filtered_count == 0) {
        if (sd->entries && gtk_revealer_get_reveal_child(GTK_REVEALER(sd->search_revealer))) {
            GtkWidget *no_results = gtk_label_new("No results");
            gtk_widget_add_css_class(no_results, "dim-label");
            gtk_widget_set_margin_top(no_results, 8);
            gtk_widget_set_margin_bottom(no_results, 8);
            gtk_list_box_append(GTK_LIST_BOX(sd->list_box), no_results);
        } else if (!sd->entries || sd->filtered_count == 0) {
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

    gboolean has_any_color = FALSE;
    if (sd->active_group < 0 && sd->groups && JSON_NODE_TYPE(sd->groups) == JSON_NODE_ARRAY) {
        JsonArray *garr = json_node_get_array(sd->groups);
        for (guint gi = 0; gi < json_array_get_length(garr); gi++) {
            JsonObject *gobj = json_array_get_object_element(garr, gi);
            if (json_object_has_member(gobj, "color") && !json_object_get_null_member(gobj, "color")) {
                const char *c = json_object_get_string_member(gobj, "color");
                if (c && *c) { has_any_color = TRUE; break; }
            }
        }
    }

    int end = sd->visible_offset + MAX_VISIBLE_ENTRIES;
    if (end > sd->filtered_count) end = sd->filtered_count;

    if (sd->visible_offset > 0) {
        GtkWidget *more_up = gtk_label_new("\xc2\xb7\xc2\xb7\xc2\xb7");
        gtk_widget_add_css_class(more_up, "scroll-indicator");
        gtk_list_box_append(GTK_LIST_BOX(sd->list_box), more_up);
        GtkListBoxRow *ind_row = GTK_LIST_BOX_ROW(gtk_widget_get_parent(more_up));
        if (ind_row) gtk_list_box_row_set_selectable(ind_row, FALSE);
    }

    for (int vi = sd->visible_offset; vi < end; vi++) {
        int i = sd->filtered_indices[vi];
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

        GtkWidget *row_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
        gtk_widget_add_css_class(row_box, "entry-row");
        gtk_widget_set_margin_start(row_box, 4);
        gtk_widget_set_margin_end(row_box, 4);
        gtk_widget_set_margin_top(row_box, 4);
        gtk_widget_set_margin_bottom(row_box, 4);
        gtk_widget_set_size_request(row_box, -1, 54);

        int display_idx = vi - sd->visible_offset;
        char lbl_char = label_char_for_index(display_idx);
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

        if (has_any_color) {
            const char *gc = NULL;
            if (json_object_has_member(obj, "group_index") &&
                !json_object_get_null_member(obj, "group_index")) {
                int gi = (int)json_object_get_int_member(obj, "group_index");
                gc = group_color_for_index(sd, gi);
            }
            GtkWidget *dot = gtk_label_new("\xe2\x97\x8f");
            gtk_widget_add_css_class(dot, "group-color-dot");
            gtk_widget_set_valign(dot, GTK_ALIGN_CENTER);
            gtk_widget_set_margin_start(dot, 6);
            if (gc) {
                char css_class[32];
                g_snprintf(css_class, sizeof(css_class), "gc-%s", gc);
                gtk_widget_add_css_class(dot, css_class);
            } else {
                gtk_widget_set_opacity(dot, 0);
            }
            gtk_box_append(GTK_BOX(row_box), dot);
        }

        GtkWidget *icon = gtk_image_new_from_icon_name(icon_for_type(type));
        gtk_image_set_pixel_size(GTK_IMAGE(icon), 20);
        gtk_widget_add_css_class(icon, "entry-icon");
        gtk_widget_set_valign(icon, GTK_ALIGN_CENTER);
        gtk_widget_set_margin_start(icon, 12);
        gtk_widget_set_margin_end(icon, 10);
        gtk_box_append(GTK_BOX(row_box), icon);

        GtkWidget *text_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
        gtk_widget_set_hexpand(text_box, TRUE);
        gtk_widget_set_valign(text_box, GTK_ALIGN_CENTER);

        GtkWidget *name_label = gtk_label_new(name);
        gtk_widget_add_css_class(name_label, "entry-name");
        gtk_label_set_xalign(GTK_LABEL(name_label), 0);
        gtk_label_set_ellipsize(GTK_LABEL(name_label), PANGO_ELLIPSIZE_END);
        gtk_box_append(GTK_BOX(text_box), name_label);

        if (subtitle && *subtitle) {
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

        g_object_set_data(G_OBJECT(row_box), "entry-index", GINT_TO_POINTER(i));
        gtk_list_box_append(GTK_LIST_BOX(sd->list_box), row_box);
    }

    if (end < sd->filtered_count) {
        GtkWidget *more_down = gtk_label_new("\xc2\xb7\xc2\xb7\xc2\xb7");
        gtk_widget_add_css_class(more_down, "scroll-indicator");
        gtk_list_box_append(GTK_LIST_BOX(sd->list_box), more_down);
        GtkListBoxRow *ind_row = GTK_LIST_BOX_ROW(gtk_widget_get_parent(more_down));
        if (ind_row) gtk_list_box_row_set_selectable(ind_row, FALSE);
    }

    int sel_row = sd->selected_filtered - sd->visible_offset;
    if (sd->visible_offset > 0) sel_row++;
    GtkListBoxRow *row = gtk_list_box_get_row_at_index(GTK_LIST_BOX(sd->list_box), sel_row);
    if (row)
        gtk_list_box_select_row(GTK_LIST_BOX(sd->list_box), row);
}

static gboolean on_entry_scroll(GtkEventControllerScroll *ctrl, double dx, double dy, gpointer data)
{
    (void)ctrl; (void)dx;
    SearchData *sd = data;
    if (sd->filtered_count == 0) return TRUE;
    int step = dy > 0 ? 2 : -2;
    int next = sd->selected_filtered + step;
    if (next < 0) next = 0;
    if (next >= sd->filtered_count) next = sd->filtered_count - 1;
    sd->selected_filtered = next;
    if (next < sd->visible_offset)
        sd->visible_offset = next;
    else if (next >= sd->visible_offset + MAX_VISIBLE_ENTRIES)
        sd->visible_offset = next - MAX_VISIBLE_ENTRIES + 1;
    rebuild_visible_list(sd);
    return TRUE;
}

static gboolean on_group_scroll(GtkEventControllerScroll *ctrl, double dx, double dy, gpointer data)
{
    (void)ctrl;
    SearchData *sd = data;
    GtkAdjustment *adj = gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(sd->group_scroll));
    double val = gtk_adjustment_get_value(adj);
    double step = (dx != 0.0) ? dx * 30.0 : dy * 30.0;
    double upper = gtk_adjustment_get_upper(adj);
    double page = gtk_adjustment_get_page_size(adj);
    double target = val + step;
    if (target < 0) target = 0;
    if (target > upper - page) target = upper - page;
    gtk_adjustment_set_value(adj, target);
    return TRUE;
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
    scroll_to_active_chip(sd);
    gtk_widget_grab_focus(sd->list_box);
}

GtkWidget *solock_search_view_new(SolockApp *app)
{
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    GtkWidget *search_revealer = gtk_revealer_new();
    gtk_revealer_set_transition_type(GTK_REVEALER(search_revealer), GTK_REVEALER_TRANSITION_TYPE_SLIDE_DOWN);
    gtk_revealer_set_transition_duration(GTK_REVEALER(search_revealer), 0);
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
                                   GTK_POLICY_EXTERNAL, GTK_POLICY_NEVER);

    GtkWidget *group_bar = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 3);
    gtk_widget_set_margin_start(group_bar, 6);
    gtk_widget_set_margin_end(group_bar, 6);
    gtk_widget_set_margin_top(group_bar, 6);
    gtk_widget_set_margin_bottom(group_bar, 2);
    gtk_scrolled_window_set_child(GTK_SCROLLED_WINDOW(group_scroll), group_bar);
    gtk_widget_set_visible(group_scroll, FALSE);
    gtk_box_append(GTK_BOX(box), group_scroll);

    GtkWidget *list_box = gtk_list_box_new();
    gtk_list_box_set_selection_mode(GTK_LIST_BOX(list_box), GTK_SELECTION_SINGLE);
    gtk_widget_add_css_class(list_box, "entry-list");
    gtk_widget_add_css_class(list_box, "keyboard-nav");
    gtk_widget_set_size_request(list_box, 320, -1);
    gtk_box_append(GTK_BOX(box), list_box);

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

    GtkEventController *entry_scroll_ctrl = GTK_EVENT_CONTROLLER(
        gtk_event_controller_scroll_new(GTK_EVENT_CONTROLLER_SCROLL_VERTICAL));
    g_signal_connect(entry_scroll_ctrl, "scroll", G_CALLBACK(on_entry_scroll), sd);
    gtk_widget_add_controller(list_box, entry_scroll_ctrl);

    GtkEventController *group_scroll_ctrl = GTK_EVENT_CONTROLLER(
        gtk_event_controller_scroll_new(GTK_EVENT_CONTROLLER_SCROLL_HORIZONTAL));
    g_signal_connect(group_scroll_ctrl, "scroll", G_CALLBACK(on_group_scroll), sd);
    gtk_widget_add_controller(group_scroll, group_scroll_ctrl);

    return box;
}

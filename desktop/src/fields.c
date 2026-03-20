#include "solock-desktop.h"
#include <string.h>

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);
extern GtkWidget    *solock_app_get_popup(SolockApp *app);

static const char LABEL_CHARS[] = "asdfghjkl";
static const int  LABEL_CHARS_LEN = 9;

static void *active_detail_data = NULL;

typedef struct {
    char *label;
    char *value;
    gboolean sensitive;
} FieldInfo;

typedef struct {
    SolockApp *app;
    JsonNode  *entry;
    guint      totp_timer;
    GtkWidget *totp_code_label;
    GtkWidget *totp_bar;
    GtkWidget *totp_hint_label;
    GtkWidget *box;
    GtkWidget *fields_box;
    gboolean   label_mode;
    FieldInfo *fields;
    int        field_count;
} DetailData;

static const char *human_label(const char *key)
{
    if (g_strcmp0(key, "password") == 0)   return "Password";
    if (g_strcmp0(key, "username") == 0)   return "Username";
    if (g_strcmp0(key, "site") == 0)       return "Website";
    if (g_strcmp0(key, "number") == 0)     return "Card Number";
    if (g_strcmp0(key, "cvv") == 0)        return "CVV";
    if (g_strcmp0(key, "expiry") == 0)     return "Expiry";
    if (g_strcmp0(key, "cardholder") == 0) return "Cardholder";
    if (g_strcmp0(key, "content") == 0)    return "Content";
    if (g_strcmp0(key, "notes") == 0)      return "Note";
    return key;
}

static gboolean is_hidden_field(const char *key)
{
    return g_strcmp0(key, "totp_secret") == 0 || g_strcmp0(key, "secret") == 0;
}

static gboolean is_sensitive_field(const char *key)
{
    return g_strcmp0(key, "password") == 0 ||
           g_strcmp0(key, "cvv") == 0 ||
           g_strcmp0(key, "number") == 0;
}

static int field_priority(const char *key)
{
    static const char *order[] = {
        "username", "password", "totp", "site",
        "cardholder", "number", "cvv", "expiry",
        "content", "notes", NULL
    };
    for (int i = 0; order[i]; i++) {
        if (g_strcmp0(key, order[i]) == 0)
            return i;
    }
    return 100;
}

static int compare_field_keys(gconstpointer a, gconstpointer b)
{
    return field_priority((const char *)a) - field_priority((const char *)b);
}

static const char *type_display_name(const char *type)
{
    if (g_strcmp0(type, "password") == 0) return "Password";
    if (g_strcmp0(type, "card") == 0)     return "Payment Card";
    if (g_strcmp0(type, "note") == 0)     return "Secure Note";
    if (g_strcmp0(type, "totp") == 0)     return "Authenticator";
    return type;
}

typedef struct {
    char *value;
    int clear_seconds;
    gboolean use_wtype;
} PasteJob;

static gboolean do_paste_delayed(gpointer data)
{
    PasteJob *job = data;
    if (job->use_wtype && solock_wtype_available()) {
        solock_wtype_type(job->value, NULL);
    } else {
        solock_clipboard_copy(job->value, job->clear_seconds, NULL);
    }
    g_free(job->value);
    g_free(job);
    return G_SOURCE_REMOVE;
}

static void do_paste_value(SolockApp *app, const char *value)
{
    SolockConfig *config = solock_app_get_config(app);
    const char *method = solock_config_get_paste_method(config);

    GtkWidget *popup = solock_app_get_popup(app);
    solock_popup_hide(popup);

    PasteJob *job = g_new0(PasteJob, 1);
    job->value = g_strdup(value);
    job->clear_seconds = solock_config_get_clipboard_clear_seconds(config);
    job->use_wtype = g_strcmp0(method, "wtype") == 0;

    g_timeout_add(150, do_paste_delayed, job);
}

typedef struct {
    SolockApp *app;
    char *value;
    GtkWidget *value_label;
} FieldClickData;

static void field_click_data_free(gpointer data)
{
    FieldClickData *fcd = data;
    g_free(fcd->value);
    g_free(fcd);
}

static void on_field_clicked(GtkGestureClick *gesture, int n_press, double x, double y, gpointer data)
{
    (void)gesture; (void)n_press; (void)x; (void)y;
    FieldClickData *fcd = data;
    do_paste_value(fcd->app, fcd->value);
}

typedef struct {
    SolockApp *app;
    char *code;
    GtkWidget *label;
} TotpClickData;

static void totp_click_data_free(gpointer data)
{
    TotpClickData *tcd = data;
    g_free(tcd->code);
    g_free(tcd);
}

static void on_totp_clicked(GtkGestureClick *gesture, int n_press, double x, double y, gpointer data)
{
    (void)gesture; (void)n_press; (void)x; (void)y;
    TotpClickData *tcd = data;

    if (tcd->code && *tcd->code) {
        SolockConfig *config = solock_app_get_config(tcd->app);
        int clear = solock_config_get_clipboard_clear_seconds(config);
        solock_clipboard_copy(tcd->code, clear, NULL);
        do_paste_value(tcd->app, tcd->code);
    } else {
        GtkWidget *popup = solock_app_get_popup(tcd->app);
        solock_popup_hide(popup);
    }
}

static const char *get_totp_secret(JsonObject *obj)
{
    const char *type = json_object_get_string_member(obj, "type");
    JsonObject *fields = json_object_get_object_member(obj, "fields");
    if (!fields) return NULL;

    if (g_strcmp0(type, "password") == 0 && json_object_has_member(fields, "totp_secret"))
        return json_object_get_string_member(fields, "totp_secret");
    if (g_strcmp0(type, "totp") == 0 && json_object_has_member(fields, "secret"))
        return json_object_get_string_member(fields, "secret");
    return NULL;
}

static gboolean update_totp(gpointer data)
{
    DetailData *dd = data;
    JsonObject *obj = json_node_get_object(dd->entry);
    const char *secret = get_totp_secret(obj);
    if (!secret || !*secret) return G_SOURCE_CONTINUE;

    SolockClient *client = solock_app_get_client(dd->app);
    JsonNode *result = solock_client_generate_totp(client, secret, 0, 0, NULL);
    if (!result) return G_SOURCE_CONTINUE;

    JsonObject *res = json_node_get_object(result);
    const char *code = json_object_get_string_member(res, "code");
    gint64 remaining = json_object_get_int_member(res, "remaining");

    gsize code_len = strlen(code);
    char *formatted = NULL;
    if (code_len == 6) {
        formatted = g_strdup_printf("%.3s %.3s", code, code + 3);
    } else {
        formatted = g_strdup(code);
    }
    gtk_label_set_text(GTK_LABEL(dd->totp_code_label), formatted);
    g_free(formatted);

    TotpClickData *tcd = g_object_get_data(G_OBJECT(dd->totp_code_label), "totp-click-data");
    if (tcd) {
        g_free(tcd->code);
        tcd->code = g_strdup(code);
    }

    gtk_level_bar_set_value(GTK_LEVEL_BAR(dd->totp_bar), (double)remaining / 30.0);

    gtk_widget_remove_css_class(dd->totp_bar, "totp-ok");
    gtk_widget_remove_css_class(dd->totp_bar, "totp-warn");
    gtk_widget_remove_css_class(dd->totp_bar, "totp-danger");
    if (remaining >= 10)
        gtk_widget_add_css_class(dd->totp_bar, "totp-ok");
    else if (remaining >= 5)
        gtk_widget_add_css_class(dd->totp_bar, "totp-warn");
    else
        gtk_widget_add_css_class(dd->totp_bar, "totp-danger");

    json_node_unref(result);
    return G_SOURCE_CONTINUE;
}

static void on_detail_destroy(GtkWidget *widget, gpointer data)
{
    (void)widget;
    DetailData *dd = data;

    if (active_detail_data == dd)
        active_detail_data = NULL;

    if (dd->totp_timer > 0) {
        g_source_remove(dd->totp_timer);
        dd->totp_timer = 0;
    }

    if (dd->entry) {
        json_node_unref(dd->entry);
        dd->entry = NULL;
    }

    if (dd->fields) {
        for (int i = 0; i < dd->field_count; i++) {
            g_free(dd->fields[i].label);
            g_free(dd->fields[i].value);
        }
        g_free(dd->fields);
    }

    g_free(dd);
}

static void update_label_hints(DetailData *dd);

static gboolean on_key_pressed(GtkEventControllerKey *ctrl, guint keyval,
                                guint keycode, GdkModifierType state, gpointer data)
{
    (void)ctrl; (void)keycode;
    DetailData *dd = data;

    if (keyval == GDK_KEY_Control_L || keyval == GDK_KEY_Control_R) {
        dd->label_mode = TRUE;
        update_label_hints(dd);
        return TRUE;
    }

    if (state & GDK_CONTROL_MASK) {
        char ch = (char)gdk_keyval_to_unicode(keyval);
        if (ch) {
            for (int i = 0; i < dd->field_count && i < LABEL_CHARS_LEN; i++) {
                if (LABEL_CHARS[i] == ch) {
                    do_paste_value(dd->app, dd->fields[i].value);
                    return TRUE;
                }
            }
            int totp_idx = dd->field_count;
            if (dd->totp_code_label && totp_idx < LABEL_CHARS_LEN && LABEL_CHARS[totp_idx] == ch) {
                TotpClickData *tcd = g_object_get_data(G_OBJECT(dd->totp_code_label), "totp-click-data");
                if (tcd && tcd->code && *tcd->code) {
                    SolockConfig *config = solock_app_get_config(dd->app);
                    int clear = solock_config_get_clipboard_clear_seconds(config);
                    solock_clipboard_copy(tcd->code, clear, NULL);
                    do_paste_value(dd->app, tcd->code);
                }
                return TRUE;
            }
        }
    }

    if (keyval == GDK_KEY_Return || keyval == GDK_KEY_KP_Enter) {
        if (dd->field_count == 1 && dd->fields[0].value && *dd->fields[0].value) {
            do_paste_value(dd->app, dd->fields[0].value);
            return TRUE;
        }
    }

    return FALSE;
}

static void on_key_released(GtkEventControllerKey *ctrl, guint keyval,
                             guint keycode, GdkModifierType state, gpointer data)
{
    (void)ctrl; (void)keycode; (void)state;
    DetailData *dd = data;

    if (keyval == GDK_KEY_Control_L || keyval == GDK_KEY_Control_R) {
        dd->label_mode = FALSE;
        update_label_hints(dd);
    }
}

static GtkWidget *make_field_row(DetailData *dd, int idx, const char *label_text,
                                  const char *display_value, const char *real_value,
                                  gboolean is_totp)
{
    GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
    gtk_widget_add_css_class(row, "detail-field-row");
    gtk_widget_set_cursor_from_name(row, "pointer");

    GtkWidget *hint_label = NULL;
    if (idx >= 0 && idx < LABEL_CHARS_LEN) {
        char lbl_str[2] = { LABEL_CHARS[idx], '\0' };
        hint_label = gtk_label_new(lbl_str);
        gtk_widget_add_css_class(hint_label, "label-hint");
        gtk_widget_add_css_class(hint_label, "label-hint-animated");
        gtk_box_append(GTK_BOX(row), hint_label);
    }

    GtkWidget *content = gtk_box_new(GTK_ORIENTATION_VERTICAL, 1);
    gtk_widget_set_hexpand(content, TRUE);
    gtk_widget_set_margin_start(content, 2);

    GtkWidget *lbl = gtk_label_new(label_text);
    gtk_widget_add_css_class(lbl, "detail-field-label");
    gtk_label_set_xalign(GTK_LABEL(lbl), 0);
    gtk_box_append(GTK_BOX(content), lbl);

    GtkWidget *val = gtk_label_new(display_value);
    gtk_widget_add_css_class(val, "detail-field-value");
    gtk_label_set_xalign(GTK_LABEL(val), 0);
    gtk_label_set_ellipsize(GTK_LABEL(val), PANGO_ELLIPSIZE_END);
    gtk_label_set_selectable(GTK_LABEL(val), FALSE);
    gtk_box_append(GTK_BOX(content), val);

    gtk_box_append(GTK_BOX(row), content);

    if (is_totp && dd->totp_bar == NULL) {
        GtkWidget *bar = gtk_level_bar_new_for_interval(0.0, 1.0);
        gtk_level_bar_set_value(GTK_LEVEL_BAR(bar), 1.0);
        gtk_widget_add_css_class(bar, "totp-progress");
        gtk_widget_set_size_request(bar, 48, -1);
        gtk_widget_set_valign(bar, GTK_ALIGN_CENTER);
        gtk_widget_set_margin_start(bar, 8);
        gtk_box_append(GTK_BOX(row), bar);
        dd->totp_bar = bar;
    }

    return row;
}

GtkWidget *solock_fields_view_new(SolockApp *app, JsonNode *entry)
{
    JsonObject *obj = json_node_get_object(entry);
    const char *name = json_object_get_string_member(obj, "name");
    const char *type = json_object_get_string_member(obj, "type");
    JsonObject *fields = json_object_get_object_member(obj, "fields");
    gboolean has_totp = json_object_get_boolean_member_with_default(obj, "has_totp", FALSE);

    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_widget_set_margin_start(box, 10);
    gtk_widget_set_margin_end(box, 10);
    gtk_widget_set_margin_top(box, 10);
    gtk_widget_set_margin_bottom(box, 8);
    gtk_widget_set_size_request(box, 280, -1);

    DetailData *dd = g_new0(DetailData, 1);
    dd->app = app;
    dd->entry = json_node_copy(entry);
    dd->box = box;
    dd->label_mode = FALSE;

    const char *site_value = NULL;
    if (fields && json_object_has_member(fields, "site"))
        site_value = json_object_get_string_member(fields, "site");

    GtkWidget *header = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_add_css_class(header, "detail-header");
    gtk_widget_set_margin_start(header, 0);
    gtk_widget_set_margin_end(header, 0);

    GtkWidget *title_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
    gtk_widget_set_hexpand(title_box, TRUE);
    if (!site_value || !*site_value)
        gtk_widget_set_valign(title_box, GTK_ALIGN_CENTER);

    GtkWidget *title = gtk_label_new(name);
    gtk_widget_add_css_class(title, "detail-name");
    gtk_label_set_xalign(GTK_LABEL(title), 0);
    gtk_label_set_ellipsize(GTK_LABEL(title), PANGO_ELLIPSIZE_END);
    gtk_box_append(GTK_BOX(title_box), title);

    if (site_value && *site_value) {
        GtkWidget *site_label = gtk_label_new(site_value);
        gtk_widget_add_css_class(site_label, "dim-label");
        gtk_widget_add_css_class(site_label, "caption");
        gtk_label_set_xalign(GTK_LABEL(site_label), 0);
        gtk_label_set_ellipsize(GTK_LABEL(site_label), PANGO_ELLIPSIZE_END);
        gtk_box_append(GTK_BOX(title_box), site_label);
    }

    gtk_box_append(GTK_BOX(header), title_box);

    GtkWidget *type_label = gtk_label_new(type_display_name(type));
    gtk_widget_add_css_class(type_label, "detail-meta");
    gtk_widget_set_valign(type_label, GTK_ALIGN_CENTER);
    gtk_box_append(GTK_BOX(header), type_label);

    gtk_box_append(GTK_BOX(box), header);

    GList *members = json_object_get_members(fields);
    members = g_list_sort(members, compare_field_keys);
    int field_count = 0;
    for (GList *l = members; l; l = l->next) {
        const char *key = l->data;
        if (is_hidden_field(key) || g_strcmp0(key, "site") == 0) continue;
        const char *val = json_object_get_string_member(fields, key);
        if (val && *val) field_count++;
    }

    int totp_idx = field_count;
    int total_items = field_count + (has_totp ? 1 : 0);
    dd->fields = g_new0(FieldInfo, field_count);
    dd->field_count = field_count;

    GtkWidget *fields_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
    dd->fields_box = fields_box;

    int idx = 0;
    for (GList *l = members; l; l = l->next) {
        const char *key = l->data;
        if (is_hidden_field(key)) continue;
        if (g_strcmp0(key, "site") == 0) continue;

        const char *value = json_object_get_string_member(fields, key);
        if (!value || !*value) continue;

        dd->fields[idx].label = g_strdup(human_label(key));
        dd->fields[idx].value = g_strdup(value);
        dd->fields[idx].sensitive = is_sensitive_field(key);

        const char *display = is_sensitive_field(key)
            ? "\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2"
            : value;

        GtkWidget *row = make_field_row(dd, idx, human_label(key), display, value, FALSE);

        FieldClickData *fcd = g_new0(FieldClickData, 1);
        fcd->app = app;
        fcd->value = g_strdup(value);
        fcd->value_label = NULL;
        g_object_set_data_full(G_OBJECT(row), "field-click-data", fcd, field_click_data_free);

        GtkGestureClick *click = GTK_GESTURE_CLICK(gtk_gesture_click_new());
        g_signal_connect(click, "pressed", G_CALLBACK(on_field_clicked), fcd);
        gtk_widget_add_controller(row, GTK_EVENT_CONTROLLER(click));

        gtk_box_append(GTK_BOX(fields_box), row);
        idx++;
    }
    g_list_free(members);

    if (has_totp) {
        const char *secret = get_totp_secret(obj);
        if (secret && *secret) {
            GtkWidget *totp_row = make_field_row(dd, totp_idx, "One-Time Password", "--- ---", "", TRUE);

            GtkWidget *content_box = gtk_widget_get_first_child(totp_row);
            if (content_box && GTK_IS_LABEL(content_box))
                content_box = gtk_widget_get_next_sibling(content_box);
            if (content_box) {
                for (GtkWidget *c = gtk_widget_get_first_child(content_box); c; c = gtk_widget_get_next_sibling(c)) {
                    if (gtk_widget_has_css_class(c, "detail-field-value")) {
                        dd->totp_code_label = c;
                        break;
                    }
                }
            }

            if (!dd->totp_code_label) {
                dd->totp_code_label = gtk_label_new("--- ---");
                gtk_widget_add_css_class(dd->totp_code_label, "detail-field-value");
            }

            GtkWidget *first = gtk_widget_get_first_child(totp_row);
            if (first && gtk_widget_has_css_class(first, "label-hint"))
                dd->totp_hint_label = first;

            TotpClickData *tcd = g_new0(TotpClickData, 1);
            tcd->app = app;
            tcd->code = g_strdup("");
            tcd->label = dd->totp_code_label;
            g_object_set_data_full(G_OBJECT(dd->totp_code_label), "totp-click-data", tcd, totp_click_data_free);

            GtkGestureClick *click = GTK_GESTURE_CLICK(gtk_gesture_click_new());
            g_signal_connect(click, "pressed", G_CALLBACK(on_totp_clicked), tcd);
            gtk_widget_add_controller(totp_row, GTK_EVENT_CONTROLLER(click));

            gtk_box_append(GTK_BOX(fields_box), totp_row);
        }
    }

    gtk_box_append(GTK_BOX(box), fields_box);

    if (has_totp && dd->totp_code_label) {
        update_totp(dd);
        dd->totp_timer = g_timeout_add_seconds(1, update_totp, dd);
    }

    GtkEventController *key_ctrl = gtk_event_controller_key_new();
    gtk_event_controller_set_propagation_phase(key_ctrl, GTK_PHASE_CAPTURE);
    g_signal_connect(key_ctrl, "key-pressed", G_CALLBACK(on_key_pressed), dd);
    g_signal_connect(key_ctrl, "key-released", G_CALLBACK(on_key_released), dd);
    gtk_widget_add_controller(box, key_ctrl);
    gtk_widget_set_focusable(box, TRUE);

    (void)total_items;

    g_signal_connect(box, "destroy", G_CALLBACK(on_detail_destroy), dd);
    g_signal_connect_swapped(box, "map", G_CALLBACK(gtk_widget_grab_focus), box);

    active_detail_data = dd;

    return box;
}

void solock_fields_reset_label_mode(void)
{
    if (!active_detail_data) return;
    DetailData *dd = active_detail_data;
    dd->label_mode = FALSE;
    update_label_hints(dd);
}

static void update_label_hints(DetailData *dd)
{
    if (!dd->fields_box) return;

    for (GtkWidget *row = gtk_widget_get_first_child(dd->fields_box);
         row != NULL;
         row = gtk_widget_get_next_sibling(row)) {

        GtkWidget *hint = gtk_widget_get_first_child(row);
        if (hint && GTK_IS_LABEL(hint) &&
            gtk_widget_has_css_class(hint, "label-hint")) {
            if (dd->label_mode)
                gtk_widget_add_css_class(hint, "label-hint-visible");
            else
                gtk_widget_remove_css_class(hint, "label-hint-visible");
        }
    }

    if (dd->totp_hint_label) {
        if (dd->label_mode)
            gtk_widget_add_css_class(dd->totp_hint_label, "label-hint-visible");
        else
            gtk_widget_remove_css_class(dd->totp_hint_label, "label-hint-visible");
    }
}

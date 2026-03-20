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
    GtkWidget *totp_hint_revealer;
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
    if (g_strcmp0(key, "notes") == 0)      return "Notes";
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
    GtkWidget *label;
    char *original_text;
    gboolean destroyed;
} CopiedFeedback;

static void on_feedback_label_destroy(GtkWidget *widget, gpointer data)
{
    (void)widget;
    CopiedFeedback *fb = data;
    fb->destroyed = TRUE;
}

static gboolean revert_label(gpointer data)
{
    CopiedFeedback *fb = data;
    if (!fb->destroyed) {
        gtk_label_set_text(GTK_LABEL(fb->label), fb->original_text);
        gtk_widget_remove_css_class(fb->label, "copied-feedback");
    }
    g_free(fb->original_text);
    g_free(fb);
    return G_SOURCE_REMOVE;
}

static void show_copied_feedback(GtkWidget *value_label)
{
    CopiedFeedback *fb = g_new0(CopiedFeedback, 1);
    fb->label = value_label;
    fb->original_text = g_strdup(gtk_label_get_text(GTK_LABEL(value_label)));
    fb->destroyed = FALSE;
    gtk_label_set_text(GTK_LABEL(value_label), "Copied!");
    gtk_widget_add_css_class(value_label, "copied-feedback");
    g_signal_connect(value_label, "destroy", G_CALLBACK(on_feedback_label_destroy), fb);
    g_timeout_add(1500, revert_label, fb);
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
    else if (remaining >= 4)
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

GtkWidget *solock_fields_view_new(SolockApp *app, JsonNode *entry)
{
    JsonObject *obj = json_node_get_object(entry);
    const char *name = json_object_get_string_member(obj, "name");
    const char *type = json_object_get_string_member(obj, "type");
    JsonObject *fields = json_object_get_object_member(obj, "fields");
    gboolean has_totp = json_object_get_boolean_member_with_default(obj, "has_totp", FALSE);

    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_widget_set_margin_start(box, 12);
    gtk_widget_set_margin_end(box, 12);
    gtk_widget_set_margin_top(box, 10);
    gtk_widget_set_margin_bottom(box, 10);
    gtk_widget_set_size_request(box, 280, -1);

    DetailData *dd = g_new0(DetailData, 1);
    dd->app = app;
    dd->entry = json_node_copy(entry);
    dd->box = box;
    dd->label_mode = FALSE;

    GtkWidget *header = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
    gtk_widget_set_margin_bottom(header, 6);

    GtkWidget *title = gtk_label_new(name);
    gtk_widget_add_css_class(title, "detail-name");
    gtk_label_set_xalign(GTK_LABEL(title), 0);
    gtk_label_set_ellipsize(GTK_LABEL(title), PANGO_ELLIPSIZE_END);
    gtk_widget_set_hexpand(title, TRUE);
    gtk_box_append(GTK_BOX(header), title);

    GtkWidget *type_label = gtk_label_new(type_display_name(type));
    gtk_widget_add_css_class(type_label, "detail-meta");
    gtk_widget_set_valign(type_label, GTK_ALIGN_CENTER);
    gtk_box_append(GTK_BOX(header), type_label);

    gtk_box_append(GTK_BOX(box), header);

    if (has_totp) {
        const char *secret = get_totp_secret(obj);
        if (secret && *secret) {
            GtkWidget *totp_section = gtk_box_new(GTK_ORIENTATION_VERTICAL, 4);
            gtk_widget_add_css_class(totp_section, "totp-section");
            gtk_widget_set_margin_top(totp_section, 4);
            gtk_widget_set_margin_bottom(totp_section, 12);

            GtkWidget *totp_header_box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);

            GtkWidget *totp_hint_revealer = gtk_revealer_new();
            gtk_revealer_set_transition_type(GTK_REVEALER(totp_hint_revealer), GTK_REVEALER_TRANSITION_TYPE_CROSSFADE);
            gtk_revealer_set_transition_duration(GTK_REVEALER(totp_hint_revealer), 100);
            gtk_revealer_set_reveal_child(GTK_REVEALER(totp_hint_revealer), FALSE);
            gtk_widget_set_hexpand(totp_hint_revealer, FALSE);
            gtk_box_append(GTK_BOX(totp_header_box), totp_hint_revealer);

            GtkWidget *totp_header = gtk_label_new("One-Time Password");
            gtk_widget_add_css_class(totp_header, "dim-label");
            gtk_widget_add_css_class(totp_header, "detail-field-label");
            gtk_label_set_xalign(GTK_LABEL(totp_header), 0);
            gtk_box_append(GTK_BOX(totp_header_box), totp_header);

            gtk_box_append(GTK_BOX(totp_section), totp_header_box);

            GtkWidget *totp_code = gtk_label_new("--- ---");
            gtk_widget_add_css_class(totp_code, "totp-code");
            gtk_label_set_xalign(GTK_LABEL(totp_code), 0);
            gtk_label_set_selectable(GTK_LABEL(totp_code), FALSE);
            gtk_box_append(GTK_BOX(totp_section), totp_code);

            GtkWidget *totp_bar = gtk_level_bar_new_for_interval(0.0, 1.0);
            gtk_level_bar_set_value(GTK_LEVEL_BAR(totp_bar), 1.0);
            gtk_widget_add_css_class(totp_bar, "totp-progress");
            gtk_widget_set_margin_top(totp_bar, 4);
            gtk_box_append(GTK_BOX(totp_section), totp_bar);

            TotpClickData *tcd = g_new0(TotpClickData, 1);
            tcd->app = app;
            tcd->code = g_strdup("");
            tcd->label = totp_code;
            g_object_set_data_full(G_OBJECT(totp_code), "totp-click-data", tcd, totp_click_data_free);

            GtkGestureClick *click = GTK_GESTURE_CLICK(gtk_gesture_click_new());
            g_signal_connect(click, "pressed", G_CALLBACK(on_totp_clicked), tcd);
            gtk_widget_add_controller(totp_section, GTK_EVENT_CONTROLLER(click));
            gtk_widget_set_cursor_from_name(totp_section, "pointer");

            gtk_box_append(GTK_BOX(box), totp_section);

            dd->totp_code_label = totp_code;
            dd->totp_bar = totp_bar;
            dd->totp_hint_revealer = totp_hint_revealer;

            GtkWidget *sep2 = gtk_separator_new(GTK_ORIENTATION_HORIZONTAL);
            gtk_widget_set_margin_bottom(sep2, 8);
            gtk_box_append(GTK_BOX(box), sep2);
        }
    }

    GList *members = json_object_get_members(fields);
    int field_count = 0;
    for (GList *l = members; l; l = l->next) {
        const char *key = l->data;
        if (!is_hidden_field(key)) field_count++;
    }

    dd->fields = g_new0(FieldInfo, field_count);
    dd->field_count = field_count;

    if (dd->totp_hint_revealer && field_count < LABEL_CHARS_LEN) {
        char lbl_str[2] = { LABEL_CHARS[field_count], '\0' };
        GtkWidget *hint = gtk_label_new(lbl_str);
        gtk_widget_add_css_class(hint, "label-hint");
        gtk_revealer_set_child(GTK_REVEALER(dd->totp_hint_revealer), hint);
    }

    GtkWidget *fields_box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    dd->fields_box = fields_box;

    int idx = 0;
    for (GList *l = members; l; l = l->next) {
        const char *key = l->data;
        if (is_hidden_field(key)) continue;

        const char *value = json_object_get_string_member(fields, key);
        if (!value) value = "";

        dd->fields[idx].label = g_strdup(human_label(key));
        dd->fields[idx].value = g_strdup(value);
        dd->fields[idx].sensitive = is_sensitive_field(key);

        GtkWidget *row = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
        gtk_widget_add_css_class(row, "detail-field-row");
        gtk_widget_set_margin_top(row, 8);
        gtk_widget_set_margin_bottom(row, 8);

        GtkWidget *row_header = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);

        GtkWidget *hint_revealer = gtk_revealer_new();
        gtk_revealer_set_transition_type(GTK_REVEALER(hint_revealer), GTK_REVEALER_TRANSITION_TYPE_CROSSFADE);
        gtk_revealer_set_transition_duration(GTK_REVEALER(hint_revealer), 100);
        gtk_revealer_set_reveal_child(GTK_REVEALER(hint_revealer), FALSE);
        gtk_widget_set_hexpand(hint_revealer, FALSE);
        if (idx < LABEL_CHARS_LEN) {
            char lbl_str[2] = { LABEL_CHARS[idx], '\0' };
            GtkWidget *hint = gtk_label_new(lbl_str);
            gtk_widget_add_css_class(hint, "label-hint");
            gtk_revealer_set_child(GTK_REVEALER(hint_revealer), hint);
        }
        gtk_box_append(GTK_BOX(row_header), hint_revealer);

        GtkWidget *field_label = gtk_label_new(human_label(key));
        gtk_widget_add_css_class(field_label, "detail-field-label");
        gtk_widget_add_css_class(field_label, "dim-label");
        gtk_label_set_xalign(GTK_LABEL(field_label), 0);
        gtk_box_append(GTK_BOX(row_header), field_label);
        gtk_box_append(GTK_BOX(row), row_header);

        const char *display = is_sensitive_field(key) ? "\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2\xe2\x80\xa2" : value;
        GtkWidget *value_label = gtk_label_new(display);
        gtk_widget_add_css_class(value_label, "detail-field-value");
        gtk_label_set_xalign(GTK_LABEL(value_label), 0);
        gtk_label_set_ellipsize(GTK_LABEL(value_label), PANGO_ELLIPSIZE_END);
        gtk_label_set_selectable(GTK_LABEL(value_label), FALSE);
        gtk_widget_set_margin_start(value_label, 0);
        gtk_box_append(GTK_BOX(row), value_label);

        FieldClickData *fcd = g_new0(FieldClickData, 1);
        fcd->app = app;
        fcd->value = g_strdup(value);
        fcd->value_label = value_label;
        g_object_set_data_full(G_OBJECT(row), "field-click-data", fcd, field_click_data_free);

        GtkGestureClick *click = GTK_GESTURE_CLICK(gtk_gesture_click_new());
        g_signal_connect(click, "pressed", G_CALLBACK(on_field_clicked), fcd);
        gtk_widget_add_controller(row, GTK_EVENT_CONTROLLER(click));
        gtk_widget_set_cursor_from_name(row, "pointer");

        gtk_box_append(GTK_BOX(fields_box), row);
        idx++;
    }
    g_list_free(members);

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

        GtkWidget *row_header = gtk_widget_get_first_child(row);
        if (!row_header) continue;

        GtkWidget *hint_revealer = gtk_widget_get_first_child(row_header);
        if (hint_revealer && GTK_IS_REVEALER(hint_revealer)) {
            gtk_revealer_set_reveal_child(GTK_REVEALER(hint_revealer), dd->label_mode);
        }
    }

    if (dd->totp_hint_revealer)
        gtk_revealer_set_reveal_child(GTK_REVEALER(dd->totp_hint_revealer), dd->label_mode);
}

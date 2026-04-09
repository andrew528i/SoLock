#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);

#define STATE_RESET_SECONDS 30

typedef struct {
    SolockApp  *app;
    GtkWidget  *window;
    GtkWidget  *stack;
    GtkWidget  *unlock_view;
    GtkWidget  *search_view;
    GtkWidget  *detail_view;
    char       *remembered_page;
    guint       reset_timer;
} PopupData;

static PopupData *popup_data = NULL;

static void remove_detail_view(PopupData *pd)
{
    if (pd->detail_view) {
        gtk_stack_remove(GTK_STACK(pd->stack), pd->detail_view);
        pd->detail_view = NULL;
    }
}

static void ensure_unlock_view(PopupData *pd)
{
    if (!pd->unlock_view) {
        pd->unlock_view = solock_unlock_view_new(pd->app);
        gtk_stack_add_named(GTK_STACK(pd->stack), pd->unlock_view, "unlock");
    }
}

static void ensure_search_view(PopupData *pd)
{
    if (!pd->search_view) {
        pd->search_view = solock_search_view_new(pd->app);
        gtk_stack_add_named(GTK_STACK(pd->stack), pd->search_view, "search");
    }
}

static void stop_reset_timer(PopupData *pd)
{
    if (pd->reset_timer > 0) {
        g_source_remove(pd->reset_timer);
        pd->reset_timer = 0;
    }
}

static gboolean on_reset_timer(gpointer data)
{
    PopupData *pd = data;
    pd->reset_timer = 0;

    g_free(pd->remembered_page);

    SolockClient *client = solock_app_get_client(pd->app);
    if (solock_client_is_locked(client)) {
        pd->remembered_page = g_strdup("unlock");
    } else {
        pd->remembered_page = g_strdup("search");
    }

    remove_detail_view(pd);

    return G_SOURCE_REMOVE;
}

static gboolean on_popup_key(GtkEventControllerKey *ctrl, guint keyval,
                              guint keycode, GdkModifierType state, gpointer data)
{
    (void)ctrl; (void)keycode; (void)state;
    PopupData *pd = data;

    if (keyval == GDK_KEY_Escape) {
        const char *visible = gtk_stack_get_visible_child_name(GTK_STACK(pd->stack));
        if (g_strcmp0(visible, "detail") == 0) {
            ensure_search_view(pd);
            gtk_stack_set_visible_child_name(GTK_STACK(pd->stack), "search");
            remove_detail_view(pd);
        } else {
            solock_popup_hide(pd->window);
        }
        return TRUE;
    }
    return FALSE;
}

GtkWidget *solock_popup_new(SolockApp *app)
{
    GtkWidget *win = gtk_window_new();
    gtk_window_set_decorated(GTK_WINDOW(win), FALSE);
    gtk_window_set_default_size(GTK_WINDOW(win), -1, -1);
    gtk_window_set_application(GTK_WINDOW(win), GTK_APPLICATION(app));

    gtk_layer_init_for_window(GTK_WINDOW(win));
    gtk_layer_set_layer(GTK_WINDOW(win), GTK_LAYER_SHELL_LAYER_OVERLAY);
    gtk_layer_set_keyboard_mode(GTK_WINDOW(win), GTK_LAYER_SHELL_KEYBOARD_MODE_ON_DEMAND);

    GtkWidget *stack = gtk_stack_new();
    gtk_stack_set_transition_type(GTK_STACK(stack), GTK_STACK_TRANSITION_TYPE_NONE);
    gtk_stack_set_vhomogeneous(GTK_STACK(stack), FALSE);
    gtk_stack_set_hhomogeneous(GTK_STACK(stack), FALSE);
    gtk_window_set_child(GTK_WINDOW(win), stack);

    PopupData *pd = g_new0(PopupData, 1);
    pd->app = app;
    pd->window = win;
    pd->stack = stack;
    pd->unlock_view = NULL;
    pd->search_view = NULL;
    pd->detail_view = NULL;
    pd->remembered_page = NULL;
    pd->reset_timer = 0;
    popup_data = pd;

    GtkEventController *key_ctrl = gtk_event_controller_key_new();
    g_signal_connect(key_ctrl, "key-pressed", G_CALLBACK(on_popup_key), pd);
    gtk_widget_add_controller(win, key_ctrl);

    gtk_widget_add_css_class(win, "popup-window");

    return win;
}

void solock_popup_show(GtkWidget *popup)
{
    if (!popup_data) return;

    stop_reset_timer(popup_data);

    SolockClient *client = solock_app_get_client(popup_data->app);

    if (solock_client_is_locked(client)) {
        ensure_unlock_view(popup_data);
        gtk_stack_set_visible_child_name(GTK_STACK(popup_data->stack), "unlock");
    } else if (popup_data->remembered_page &&
               g_strcmp0(popup_data->remembered_page, "unlock") != 0) {
        if (g_strcmp0(popup_data->remembered_page, "detail") == 0 &&
            popup_data->detail_view) {
            gtk_stack_set_visible_child_name(GTK_STACK(popup_data->stack), "detail");
        } else {
            ensure_search_view(popup_data);
            gtk_stack_set_visible_child_name(GTK_STACK(popup_data->stack), "search");
        }
    } else {
        ensure_search_view(popup_data);
        gtk_stack_set_visible_child_name(GTK_STACK(popup_data->stack), "search");
    }

    solock_fields_reset_label_mode();
    gtk_widget_set_visible(popup, TRUE);
}

void solock_popup_hide(GtkWidget *popup)
{
    if (!popup_data) return;

    const char *visible = gtk_stack_get_visible_child_name(GTK_STACK(popup_data->stack));
    g_free(popup_data->remembered_page);
    popup_data->remembered_page = g_strdup(visible);

    gtk_widget_set_visible(popup, FALSE);

    popup_data->reset_timer = g_timeout_add_seconds(STATE_RESET_SECONDS,
                                                     on_reset_timer, popup_data);
}

void solock_popup_toggle(GtkWidget *popup)
{
    if (gtk_widget_get_visible(popup)) {
        solock_popup_hide(popup);
    } else {
        solock_popup_show(popup);
    }
}

void solock_popup_switch_to_search(GtkWidget *popup)
{
    (void)popup;
    if (!popup_data) return;

    remove_detail_view(popup_data);
    ensure_search_view(popup_data);
    gtk_stack_set_visible_child_name(GTK_STACK(popup_data->stack), "search");
}

static gboolean touch_entry_idle(gpointer data)
{
    char *id = data;
    if (popup_data) {
        SolockClient *client = solock_app_get_client(popup_data->app);
        JsonNode *fresh = solock_client_get_entry(client, id, NULL);
        if (fresh) json_node_unref(fresh);
    }
    g_free(id);
    return G_SOURCE_REMOVE;
}

void solock_popup_switch_to_detail(GtkWidget *popup, JsonNode *entry)
{
    (void)popup;
    if (!popup_data) return;

    JsonObject *obj = json_node_get_object(entry);
    if (json_object_has_member(obj, "id"))
        g_idle_add(touch_entry_idle, g_strdup(json_object_get_string_member(obj, "id")));

    remove_detail_view(popup_data);

    popup_data->detail_view = solock_fields_view_new(popup_data->app, entry);
    gtk_stack_add_named(GTK_STACK(popup_data->stack), popup_data->detail_view, "detail");

    GdkDisplay *display = gdk_display_get_default();
    GdkSeat *seat = gdk_display_get_default_seat(display);
    GdkDevice *keyboard = gdk_seat_get_keyboard(seat);
    if (keyboard) {
        GdkModifierType mods = gdk_device_get_modifier_state(keyboard);
        if (mods & GDK_CONTROL_MASK)
            solock_fields_set_label_mode(TRUE);
    }
    gtk_stack_set_visible_child_name(GTK_STACK(popup_data->stack), "detail");
}

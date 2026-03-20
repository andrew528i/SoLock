#include "solock-desktop.h"

extern SolockClient *solock_app_get_client(SolockApp *app);
extern SolockConfig *solock_app_get_config(SolockApp *app);

typedef struct {
    SolockApp  *app;
    GtkWidget  *window;
    GtkWidget  *stack;
    GtkWidget  *unlock_view;
    GtkWidget  *search_view;
} PopupData;

static PopupData *popup_data = NULL;

static void popup_switch_to_search(PopupData *pd)
{
    if (!pd->search_view) {
        pd->search_view = solock_search_view_new(pd->app);
        gtk_stack_add_named(GTK_STACK(pd->stack), pd->search_view, "search");
    }
    gtk_stack_set_visible_child_name(GTK_STACK(pd->stack), "search");
}

static void popup_switch_to_unlock(PopupData *pd)
{
    if (!pd->unlock_view) {
        pd->unlock_view = solock_unlock_view_new(pd->app);
        gtk_stack_add_named(GTK_STACK(pd->stack), pd->unlock_view, "unlock");
    }
    gtk_stack_set_visible_child_name(GTK_STACK(pd->stack), "unlock");
}

static gboolean on_popup_key(GtkEventControllerKey *ctrl, guint keyval,
                              guint keycode, GdkModifierType state, gpointer data)
{
    (void)ctrl; (void)keycode; (void)state;
    PopupData *pd = data;
    if (keyval == GDK_KEY_Escape) {
        solock_popup_hide(pd->window);
        return TRUE;
    }
    return FALSE;
}

GtkWidget *solock_popup_new(SolockApp *app)
{
    GtkWidget *win = gtk_window_new();
    gtk_window_set_decorated(GTK_WINDOW(win), FALSE);
    gtk_window_set_default_size(GTK_WINDOW(win), 400, 350);
    gtk_window_set_application(GTK_WINDOW(win), GTK_APPLICATION(app));

    gtk_layer_init_for_window(GTK_WINDOW(win));
    gtk_layer_set_layer(GTK_WINDOW(win), GTK_LAYER_SHELL_LAYER_OVERLAY);
    gtk_layer_set_keyboard_mode(GTK_WINDOW(win), GTK_LAYER_SHELL_KEYBOARD_MODE_EXCLUSIVE);

    GtkWidget *stack = gtk_stack_new();
    gtk_stack_set_transition_type(GTK_STACK(stack), GTK_STACK_TRANSITION_TYPE_CROSSFADE);
    gtk_stack_set_transition_duration(GTK_STACK(stack), 150);
    gtk_window_set_child(GTK_WINDOW(win), stack);

    PopupData *pd = g_new0(PopupData, 1);
    pd->app = app;
    pd->window = win;
    pd->stack = stack;
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

    SolockClient *client = solock_app_get_client(popup_data->app);
    if (solock_client_is_locked(client)) {
        popup_switch_to_unlock(popup_data);
    } else {
        popup_switch_to_search(popup_data);
    }

    gtk_widget_set_visible(popup, TRUE);
}

void solock_popup_hide(GtkWidget *popup)
{
    gtk_widget_set_visible(popup, FALSE);
}

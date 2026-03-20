project('solock-desktop', 'c',
  version: '0.1.0',
  default_options: ['warning_level=2', 'c_std=c17'],
)

gtk4 = dependency('gtk4', version: '>= 4.10')
adw = dependency('libadwaita-1', version: '>= 1.4')
layer_shell = dependency('gtk4-layer-shell-0', required: true)
json_glib = dependency('json-glib-1.0')

sources = files(
  'src/main.c',
  'src/application.c',
  'src/client.c',
  'src/config.c',
  'src/popup.c',
  'src/search.c',
  'src/fields.c',
  'src/unlock.c',
  'src/main_window.c',
  'src/dashboard.c',
  'src/vault_view.c',
  'src/entry_editor.c',
  'src/settings.c',
  'src/tray.c',
  'src/wtype.c',
  'src/clipboard.c',
)

datadir = get_option('prefix') / get_option('datadir') / 'solock-desktop'
add_project_arguments('-D_GNU_SOURCE', language: 'c')
add_project_arguments('-DSOLOCK_DATA_DIR="' + datadir + '"', language: 'c')

executable('solock-desktop',
  sources,
  dependencies: [gtk4, adw, layer_shell, json_glib],
  install: true,
)

install_data('data/style.css',
  install_dir: datadir,
)

install_data('data/solock-desktop.desktop',
  install_dir: get_option('datadir') / 'applications',
)

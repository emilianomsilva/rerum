#!/usr/bin/env bash
# Compila o Rerum (Wails + Svelte + Go in-process) e instala o atalho no sistema.
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export PATH="${PATH}:${HOME}/go/bin"
USER_BIN="${HOME}/.local/bin"
DATA_HOME="${XDG_DATA_HOME:-}"
[[ "${DATA_HOME}" = /* ]] || DATA_HOME="${HOME}/.local/share"
APPLICATIONS="${DATA_HOME}/applications"
ICON_SVG_DIR="${DATA_HOME}/icons/hicolor/scalable/apps"
DESKTOP_FILE="${APPLICATIONS}/rerum.desktop"
LAUNCHER="${USER_BIN}/rerum"
BINARY="${USER_BIN}/rerum-app"

make -C "${PROJECT_ROOT}" build
BUILD_BINARY="${PROJECT_ROOT}/wails-app/build/bin/wails-app"
LEGACY_DATA="${RERUM_LEGACY_DATA_DIR:-${PROJECT_ROOT}/data}"
if [[ -f "${LEGACY_DATA}/rerum.sqlite" && ! -e "${DATA_HOME}/rerum/rerum.sqlite" ]] && pgrep -u "$(id -u)" -x rerum-app >/dev/null; then
  echo "Feche o Rerum antes de transferir o banco anterior." >&2
  exit 1
fi
"${BUILD_BINARY}" --import-legacy-data "${LEGACY_DATA}"

mkdir -p "${USER_BIN}" "${APPLICATIONS}" "${ICON_SVG_DIR}"
install -m 755 "${BUILD_BINARY}" "${BINARY}.new"
mv -f "${BINARY}.new" "${BINARY}"
install -m 644 "${PROJECT_ROOT}/wails-app/frontend/public/rerum.svg" "${ICON_SVG_DIR}/rerum.svg"
rm -f "${DATA_HOME}/icons/hicolor/512x512/apps/rerum.png"

# Resíduos da versão anterior (PySide + backend HTTP separado).
rm -f "${USER_BIN}/rerum-backend"

cat >"${LAUNCHER}" <<LAUNCH
#!/usr/bin/env bash
exec "${BINARY}" "\$@"
LAUNCH
chmod 755 "${LAUNCHER}"

sed \
  -e "s|@LAUNCHER@|${LAUNCHER}|g" \
  "${PROJECT_ROOT}/packaging/rerum.desktop.in" >"${DESKTOP_FILE}"
chmod 644 "${DESKTOP_FILE}"

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "${APPLICATIONS}" || true
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache --force --quiet "${DATA_HOME}/icons/hicolor" || true

echo "Aplicativo instalado: ${BINARY}"
echo "Atalho instalado: ${DESKTOP_FILE}"

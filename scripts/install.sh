#!/usr/bin/env bash
# Elsabo: скачать последний релиз с GitHub или собрать через go install.
# Публичный API релизов — токен GitHub не нужен.
#
# Установка:
#   curl -fsSL https://raw.githubusercontent.com/MidasWR/Elasbo/main/scripts/install.sh | bash
#   PREFIX=~/.local/bin curl -fsSL ... | bash
#   curl -fsSL ... | sudo PREFIX=/usr/local/bin bash
# Установка и сразу запуск TUI:
#   curl -fsSL https://raw.githubusercontent.com/MidasWR/Elasbo/main/scripts/install.sh | bash -s -- --run

set -euo pipefail

RUN_AFTER=false
for _arg in "$@"; do
	case "${_arg}" in
	--run | -r) RUN_AFTER=true ;;
	esac
done

REPO="MidasWR/Elasbo"
DEFAULT_PREFIX="${HOME}/.local/bin"
PREFIX="${PREFIX:-$DEFAULT_PREFIX}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "${arch}" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*)
	echo "elsabo install: неподдерживаемая архитектура: ${arch}" >&2
	exit 1
	;;
esac

case "${os}" in
linux) goos=linux ;;
darwin) goos=darwin ;;
*)
	echo "elsabo install: неподдерживаемая ОС: ${os}" >&2
	exit 1
	;;
esac

die() {
	echo "elsabo install: $*" >&2
	exit 1
}

try_go_install_ref() {
	local ref="$1"
	command -v go >/dev/null 2>&1 || return 1
	echo "elsabo install: пробую go install github.com/${REPO}/cmd/elsabo@${ref} …" >&2
	mkdir -p "${PREFIX}" || die "не удалось создать ${PREFIX}"
	GOBIN="${PREFIX}" CGO_ENABLED=0 go install "github.com/${REPO}/cmd/elsabo@${ref}"
}

install_from_release() {
	local json tag ver name url tmp
	json=$(curl -fsSL --connect-timeout 15 "https://api.github.com/repos/${REPO}/releases/latest") || return 1
	tag=$(printf '%s' "${json}" | grep -oE '"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"' | head -1 | sed 's/.*"\([^"]*\)"$/\1/') || true
	[[ -n "${tag}" ]] || return 1
	ver="${tag#v}"
	name="elsabo_${ver}_${goos}_${arch}.tar.gz"
	url="https://github.com/${REPO}/releases/download/${tag}/${name}"

	tmp=$(mktemp -d)
	trap 'rm -rf "${tmp}"' EXIT
	if ! curl -fsSL --connect-timeout 30 "${url}" -o "${tmp}/archive.tar.gz"; then
		rm -rf "${tmp}"
		trap - EXIT
		return 1
	fi
	tar -xzf "${tmp}/archive.tar.gz" -C "${tmp}"
	mkdir -p "${PREFIX}" || die "не удалось создать ${PREFIX}"
	bin="${tmp}/elsabo"
	[[ -f "${bin}" ]] || die "в архиве нет бинарника elsabo"
	if command -v install >/dev/null 2>&1; then
		install -m 0755 "${bin}" "${PREFIX}/elsabo"
	else
		cp -f "${bin}" "${PREFIX}/elsabo"
		chmod 0755 "${PREFIX}/elsabo"
	fi
	rm -rf "${tmp}"
	trap - EXIT
}

main() {
	if install_from_release; then
		echo "Установлено: ${PREFIX}/elsabo"
	else
		echo "elsabo install: релиз недоступен или URL архива не найден, пробую go install @latest …" >&2
		try_go_install_ref "latest" || die "нужен curl и либо GitHub-релиз, либо Go (https://go.dev/dl/)"
		echo "Установлено через go: ${PREFIX}/elsabo"
	fi

	case ":${PATH}:" in
	*:"${PREFIX}":*) ;;
	*)
		echo "" >&2
		echo "Добавьте в PATH, например:" >&2
		echo "  export PATH=\"${PREFIX}:\${PATH}\"" >&2
		echo "Запуск: elsabo" >&2
		;;
	esac

	if [[ "${RUN_AFTER}" == true ]]; then
		exec "${PREFIX}/elsabo"
	fi
}

main "$@"

PYTHON ?= python3
CHANNEL ?= dev
INSTALL_DIR ?= $(HOME)/Applications/odds Dev.app
IDENTITY ?=
VERSION_BUMP ?= patch
RELEASE_FLAGS ?=
RELEASE_REPO ?= knothhe/odds
RELEASE_TAG ?=
SIGNING_P12 ?=
UPDATER_KEY ?=
SIGNING_FLAGS ?=
HOMEBREW_TAP_DIR ?=
CURRENT_VERSION ?=
EXPECTED_VERSION ?=

.PHONY: help setup run preview build package install uninstall test test-scripts check licenses homebrew-cask verify-public-update verify-native signing-list signing-config signing-upload update-keygen release release-initial release-notes
help:
	@printf '%s\n' 'make setup                     Prepare Dev icon tools' 'make build                     Build isolated, fixed-signed Dev app' 'make install                   Build and install Dev into ~/Applications' 'make uninstall                 Remove that Dev app; retain data' 'make preview                   Temporary settings; no global shortcuts/login registration' 'make check                     Go/race/UI, scripts and dependency notices' 'make verify-native             Isolated native window verification' 'make licenses                  Regenerate dependency notices' 'make homebrew-cask RELEASE_TAG=vX.Y.Z Generate from an existing public release' 'make verify-public-update CURRENT_VERSION=X.Y.Z EXPECTED_VERSION=X.Y.Z Verify public download without installing' 'make signing-list              List paired Code Signing identities' 'make signing-config IDENTITY=… Remember one local Dev identity' 'make update-keygen             Create/reuse repo-specific update key outside source' 'make signing-upload            Configure CI signing secrets interactively' 'make release-initial           Preview first release of current version' 'make release VERSION_BUMP=patch Preview a subsequent release' 'Append RELEASE_FLAGS=--apply only when intentionally publishing.'
setup:
	$(PYTHON) -m venv .venv-build
	.venv-build/bin/python3 -m pip install -r requirements-build.txt
run:
	cd source && $(PYTHON) ../scripts/go-patched.py run .
preview:
	cd source && $(PYTHON) ../scripts/go-patched.py run . --preview
build:
	$(PYTHON) scripts/macos.py build --channel "$(CHANNEL)"
package:
	$(PYTHON) scripts/macos.py package --channel "$(CHANNEL)"
install:
	$(PYTHON) scripts/macos.py install --channel "$(CHANNEL)" --destination "$(INSTALL_DIR)"
uninstall:
	$(PYTHON) scripts/macos.py uninstall --channel "$(CHANNEL)" --destination "$(INSTALL_DIR)"
signing-list:
	$(PYTHON) scripts/macos.py signing-list
signing-config:
	$(PYTHON) scripts/macos.py signing-config --identity "$(IDENTITY)"
signing-upload:
	@$(PYTHON) scripts/signing_upload.py --p12 "$(SIGNING_P12)" --updater-key "$(UPDATER_KEY)" --repo "$(RELEASE_REPO)" $(SIGNING_FLAGS)
update-keygen:
	$(PYTHON) scripts/project.py update-keygen

test:
	cd source && go test -race ./...
test-scripts:
	PYTHONDONTWRITEBYTECODE=1 $(PYTHON) -m unittest discover -s tests/scripts -v
licenses:
	$(PYTHON) scripts/licenses.py
homebrew-cask:
	$(PYTHON) scripts/homebrew.py --tag "$(RELEASE_TAG)" --tap "$(HOMEBREW_TAP_DIR)"
verify-public-update:
	$(PYTHON) scripts/verify-public-update.py --current "$(CURRENT_VERSION)" --expected "$(EXPECTED_VERSION)"
verify-native:
	$(PYTHON) scripts/verify-native.py
check: test test-scripts
	cd source && go tool mygo vet
	$(PYTHON) scripts/licenses.py --check

# Both previews share the same engine. Actions owns all formal build/upload steps.
release-initial:
	$(PYTHON) scripts/release.py initial $(RELEASE_FLAGS)
release:
	$(PYTHON) scripts/release.py $(VERSION_BUMP) $(RELEASE_FLAGS)
release-notes:
	$(PYTHON) scripts/release-notes.py "$(RELEASE_REPO)" "$(RELEASE_TAG)"

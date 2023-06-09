BUILD_CHANNEL?=local

appimage: NO_UPX=1
appimage: server-static
	cd etc/packaging/appimages && BUILD_CHANNEL=${BUILD_CHANNEL} appimage-builder --recipe rplidar-module-`uname -m`.yml
	if [ "${RELEASE_TYPE}" = "stable" ]; then \
		cd etc/packaging/appimages; \
		BUILD_CHANNEL=stable appimage-builder --recipe rplidar-module-`uname -m`.yml; \
	fi
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod 755 etc/packaging/appimages/deploy/*.AppImage

# AppImage packaging targets run in canon docker
appimage-multiarch: appimage-amd64 appimage-arm64

appimage-amd64:
	canon --arch amd64 make appimage

appimage-arm64:
	canon --arch arm64 make appimage

appimage-deploy:
	gsutil -m -h "Cache-Control: no-cache" cp etc/packaging/appimages/deploy/* gs://packages.viam.com/apps/rplidar-module/

static-release: server-static
	rm -rf etc/packaging/static/deploy/
	mkdir -p etc/packaging/static/deploy/
	cp $(BIN_OUTPUT_PATH)/rplidar-module etc/packaging/static/deploy/rplidar-module-${BUILD_CHANNEL}-`uname -m`
	if [ "${RELEASE_TYPE}" = "stable" ]; then \
		cp $(BIN_OUTPUT_PATH)/rplidar-module etc/packaging/static/deploy/rplidar-module-stable-`uname -m`; \
	fi

.PHONY: clean-appimage
clean-appimage:
	rm -rf etc/packaging/appimages/AppDir && rm -rf etc/packaging/appimages/appimage-build && rm -rf etc/packaging/appimages/deploy

rclone:
	go build

clean:
	go clean
	find . -name \*~ | xargs -r rm -f
	rm -rf build docs/public

website:
	cd docs && hugo

upload_website:	website
	./rclone sync docs/public memstore:www-rclone-org

upload:
	rsync -avz build/ www.craig-wood.com:public_html/pub/rclone/

cross:
	./cross-compile

serve:
	cd docs && hugo server -v -w

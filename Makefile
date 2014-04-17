rclone:
	go build

clean:
	go clean
	find . -name \*~ | xargs -r rm -f
	rm -rf build rclone.org/public

website:
	cd rclone.org && hugo

upload_website:	website
	./rclone sync rclone.org/public memstore:www-rclone-org

upload:
	rsync -avz build/ www.craig-wood.com:public_html/pub/rclone/

cross:
	./cross-compile

serve:
	cd rclone.org && hugo server -v -w

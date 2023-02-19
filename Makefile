
serve: 
	docker run -it --rm -p "8080:8080" -v "$(CURDIR)/src:/app" -v "$(CURDIR)/nginx.conf:/etc/nginx/nginx.conf" nginx:alpine
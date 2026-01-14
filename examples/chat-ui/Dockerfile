# Chat UI - Static file server for GoMind travel chat interface
# Uses nginx:alpine for minimal footprint (~20MB)

FROM nginx:1.25-alpine

# Remove default nginx config and static files
RUN rm -rf /usr/share/nginx/html/* /etc/nginx/conf.d/default.conf

# Copy custom nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy the static HTML file
COPY index.html /usr/share/nginx/html/

# Ensure proper permissions - this fixes the "forbidden" error
RUN chmod -R 755 /usr/share/nginx/html && \
    chown -R nginx:nginx /usr/share/nginx/html

# Expose port 80
EXPOSE 80

# Run nginx in foreground
CMD ["nginx", "-g", "daemon off;"]

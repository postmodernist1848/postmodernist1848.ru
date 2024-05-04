set -e

docker build -t 'postmodernist1848.ru-server:latest' .

if [ "$(docker ps -aq -f name="server-instance")" ]; then
    docker rm "server-instance"
fi
docker run --name "server-instance" --detach -p 80:80 -p 443:443 \
    -v /root/database.db:/database.db \
    -v /root/log.html:/log.html \
    -v /etc/letsencrypt/live/www.postmodernist1848.ru/fullchain.pem:/server.crt \
    -v /etc/letsencrypt/live/www.postmodernist1848.ru/privkey.pem:/server.key \
    postmodernist1848.ru-server:latest
docker logs -f "server-instance" &> server.log &
docker image prune -f

set -e

docker build -t 'postmodernist1848.ru-server:latest' .

if [ "$(docker ps -q -f name="server-instance")" ]; then
    docker stop "server-instance"
    docker rm "server-instance"
fi
docker run --name "server-instance" --detach -p 80:80 -p 443:443 -v /root/database.db:/database.db postmodernist1848.ru-server:latest
docker logs -f "server-instance" &> server.log &
docker image prune -f

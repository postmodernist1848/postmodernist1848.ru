set -e
RUNNING=$(docker ps --filter ancestor=postmodernist1848.ru-server:latest --format="{{.ID}}")
docker build -t 'postmodernist1848.ru-server:latest' .
[ -n "$RUNNING" ] && echo "Stopping active server instance $RUNNING" && docker stop $RUNNING
docker run --detach -p 80:80 -p 443:443 --rm postmodernist1848.ru-server:latest
docker image prune -f

# Docker compose

The docker-compose provides an app container for Voyager itself and a signer container for Clef.
To prepare your machine to run docker compose execute
```
mkdir -p voyager && cd voyager
wget -q https://raw.githubusercontent.com/yanhuangpai/voyager/master/packaging/docker/docker-compose.yml
wget -q https://raw.githubusercontent.com/yanhuangpai/voyager/master/packaging/docker/env -O .env
```
Set all configuration variables inside `.env`

`clef` is configured with `CLEF_CHAINID=5` for goerli

To configure `voyager` set:
- `VOYAGER_SWAP_ENDPOINT=https://rpc.slock.it/goerli`

Set voyager password by either setting `VOYAGER_PASSWORD` or `VOYAGER_PASSWORD_FILE`

If you want to use password file set it to
- `VOYAGER_PASSWORD_FILE=/password`

Mount password file local file system by adding
```
- ./password:/password
```
to voyager volumes inside `docker-compose.yml`

Start it with
```
docker-compose up -d
```

From logs find URL line with `on goerli you can get both goerli eth and goerli ifi from` and prefund your node
```
docker-compose logs -f voyager-1
```

Update services with
```
docker-compose pull && docker-compose up -d
```

## Running multiple Voyager nodes
It is easy to run multiple voyager nodes with docker compose by adding more services to `docker-compose.yaml`
To do so, open `docker-compose.yaml`, copy lines 3-58 and past this after line 58.
In the copied lines, replace all occurences of `voyager-1` with `voyager-2`, `clef-1` with `clef-2` and adjust the `API_ADDR` and `P2P_ADDR` and `DEBUG_API_ADDR` to respectively `1733`, `1734` and `127.0.0.1:1735`
Lastly, add your newly configured services under `volumes` (last lines), such that it looks like:
```yaml
volumes:
  clef-1:
  voyager-1:
  voyager-2:
  clef-2:
```

If you want to create more than two nodes, simply repeat the process above, ensuring that you keep unique name for your voyager and clef services and update the ports
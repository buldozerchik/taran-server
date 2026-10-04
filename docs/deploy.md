# Развёртывание сервера

## Установка скриптом (рекомендуется)

```bash
curl -fsSL https://raw.githubusercontent.com/buldozerchik/taran-server/master/scripts/install.sh | sudo bash
```

Мастер спросит:

1. **VPN на сервере** - новый WireGuard (скрипт поднимет его сам) или уже установленный VPN (Amnezia, wg-easy, 3x-ui и т.п.: указываете его `host:port`).
2. **Запуск** - `docker` или `systemd`.
3. **Порт и обфускация.**

В конце - ссылка `taran://` для приложения (и WireGuard-конфиг, если VPN новый).

Управление:

```bash
taran                      # меню: клиенты, настройки, обновление, логи, удаление
taran client add phone     # новый клиент и его ссылка
taran client list
taran client qr phone      # показать ссылку и QR ещё раз
taran client remove phone
```

Без вопросов:

```bash
curl -fsSL https://raw.githubusercontent.com/buldozerchik/taran-server/master/scripts/install.sh | \
  sudo bash -s -- -y --backend=new --method=docker
```

Все флаги: `taran --help`.

## Ручная установка

Ключ обфускации (одинаковый на сервере и клиенте): `openssl rand -hex 32`.

### Docker Compose

```yaml
services:
  taran-server:
    image: ghcr.io/buldozerchik/taran-server:latest
    network_mode: host
    restart: unless-stopped
    environment:
      - CONNECT_ADDR=127.0.0.1:51820   # ваш VPN
      - LISTEN_ADDR=0.0.0.0:53530
      - OBF_PROFILE=rtpopus3
      - OBF_KEY=<ВАШ_КЛЮЧ>
```

| Переменная | По умолчанию | |
| --- | --- | --- |
| `CONNECT_ADDR` | обязательна | адрес вашего VPN |
| `LISTEN_ADDR` | `0.0.0.0:53530` | внешний адрес |
| `MODE` | `udp` | `udp` (WireGuard) \| `tcp` (Xray/VLESS), как на клиенте |
| `OBF_PROFILE` | `none` | `none` \| `rtpopus` \| `rtpopus2` \| `rtpopus3` |
| `OBF_KEY` | - | ключ обфускации |
| `CLIENTS_FILE` | - | список разрешённых Client ID |
| `KCP_*` | - | тюнинг `MODE=tcp`, см. `docs/flags.md` |
| `DEBUG` | `false` | подробные логи |

### systemd

```bash
sudo mkdir -p /opt/taran-server
sudo curl -L -o /opt/taran-server/server \
  https://github.com/buldozerchik/taran-server/releases/latest/download/server-linux-amd64
sudo chmod +x /opt/taran-server/server
```

`/etc/systemd/system/taran-server.service`:

```ini
[Unit]
Description=Taran Server
After=network-online.target

[Service]
ExecStart=/opt/taran-server/server -listen 0.0.0.0:53530 -connect 127.0.0.1:51820 -obf-profile rtpopus3 -obf-key <ВАШ_КЛЮЧ>
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now taran-server
```

### Доступ по Client ID

Без списка подключиться может любой, у кого есть ключ обфускации. Список:

```bash
sudo CLIENTS_FILE=/opt/taran-server/clients.json \
  /opt/taran-server/server clients add $(openssl rand -hex 16) phone
```

Запуск с `-clients-file /opt/taran-server/clients.json` (Docker: `CLIENTS_FILE` + volume). На клиенте - `-client-id` или ссылка `taran://`.

## Порт

Откройте внешний порт по **UDP** (при `MODE=tcp` тоже): `sudo ufw allow 53530/udp`. Скрипт делает это сам.

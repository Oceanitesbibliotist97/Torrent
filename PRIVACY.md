# Privacy policy

*Русский текст — ниже.*

## Summary

Torrent collects **no data**. It has no servers, no accounts, no analytics, no crash reporting and no update checks. Nothing about you or your use of the app is ever sent to its authors or to anyone else.

## What stays on your computer

Everything the app stores is in its data folder — `TorrentData\` next to `Torrent.exe`, or `%APPDATA%\Torrent` if the app folder is read-only:

| File | Contents | Removed when |
|---|---|---|
| `settings.json` | Your settings. The proxy password is encrypted with Windows DPAPI and only your Windows account can decrypt it. | You delete the folder |
| `session.json` | Your transfer list: names, save folders, progress and statistics. | You remove a transfer, or on exit when "Remember transfers" is off |
| `torrents\*.torrent` | Metadata of your transfers, so magnet links need not be fetched again. | Same as above |
| `resume.db` | Which pieces were verified, so downloads resume without rechecking. | Same as above |
| `webview\` | Cache of the Microsoft WebView2 runtime that draws the interface (local files only). | You delete the folder |

The app writes **no logs**.

## Network connections

The app connects only where BitTorrent requires, according to your settings:

- **Peers** you exchange data with, **trackers** listed in your torrents, the **DHT** network and **web seeds** listed in torrents.
- **Your proxy**, if you configured one.
- **Your system DNS**, to resolve tracker names (in VPN mode, through the VPN; in proxy mode, local DNS is blocked and the proxy resolves names).

It never contacts any other server. Donation pages open in your browser only when you click them; the app sends them nothing.

## What others can see

- **Peers and trackers** see the IP address you connect from. Without a VPN or proxy, that is your real IP. The app hides its name, version and a stable peer ID (anonymous mode), but it cannot hide your address.
- **DHT nodes** see your IP and the info hashes you look up while DHT is enabled.
- **Your ISP** can see that you use encrypted peer-to-peer traffic, but not its content when encryption is required.
- **Microsoft WebView2** is a Windows system component. The app loads only local content and disables SmartScreen URL reporting; the runtime follows your Windows diagnostic-data settings.

---

# Политика приватности

## Кратко

Torrent **не собирает данных**. У него нет серверов, аккаунтов, аналитики, отчётов о сбоях и проверок обновлений. Никакие сведения о вас и об использовании приложения никогда не отправляются ни авторам, ни кому-либо ещё.

## Что хранится на вашем компьютере

Всё хранится в папке данных — `TorrentData\` рядом с `Torrent.exe` или `%APPDATA%\Torrent`, если папка приложения доступна только для чтения:

| Файл | Содержимое | Когда удаляется |
|---|---|---|
| `settings.json` | Ваши настройки. Пароль прокси зашифрован Windows DPAPI, расшифровать его может только ваша учётная запись Windows. | Когда вы удалите папку |
| `session.json` | Список загрузок: названия, папки, прогресс и статистика. | При удалении загрузки или при выходе, если «Запоминать загрузки» выключено |
| `torrents\*.torrent` | Метаданные загрузок, чтобы не получать их по magnet-ссылке заново. | Так же |
| `resume.db` | Какие части проверены, чтобы продолжать загрузку без перепроверки. | Так же |
| `webview\` | Кэш среды Microsoft WebView2, которая отрисовывает интерфейс (только локальные файлы). | Когда вы удалите папку |

Приложение **не ведёт логов**.

## Сетевые подключения

Приложение подключается только туда, куда требует BitTorrent, согласно вашим настройкам:

- к **пирам**, с которыми вы обмениваетесь данными, **трекерам** из ваших торрентов, сети **DHT** и **веб-сидам** из торрентов;
- к **вашему прокси**, если он настроен;
- к **системному DNS** для имён трекеров (в режиме VPN — через VPN; в режиме прокси локальный DNS заблокирован, имена разрешает прокси).

Ни к каким другим серверам приложение не обращается. Страницы пожертвований открываются в браузере только по вашему нажатию, и приложение ничего им не передаёт.

## Что видят другие

- **Пиры и трекеры** видят IP-адрес, с которого вы подключаетесь. Без VPN или прокси это ваш настоящий IP. Приложение скрывает своё название, версию и постоянный peer ID (анонимный режим), но не может скрыть адрес.
- **Узлы DHT** видят ваш IP и инфо-хеши, которые вы ищете, пока DHT включён.
- **Провайдер** видит, что вы используете зашифрованный P2P-трафик, но не его содержимое, если шифрование обязательно.
- **Microsoft WebView2** — системный компонент Windows. Приложение загружает только локальное содержимое и отключает проверку адресов SmartScreen; сама среда следует вашим настройкам диагностических данных Windows.

# Veltryx Vision V2

Gestion multi-caméras GigE Vision pour les opérations de chronométrage automobile. Un processus Go acquiert les images originales, détecte les passages et produit les captures et la vidéo. Next.js fournit le dashboard. Ni FFmpeg, ni GStreamer, ni Node.js ne participent au pipeline vidéo Go.

## Démarrage sous Arch Linux

Prérequis : Go 1.25+, Node.js 20.9+ pour Next.js, `pkg-config`, compilateur C. L’intégration native de cette version cible **Aravis 0.10** et x264.

```bash
sudo pacman -S --needed go nodejs npm base-devel pkgconf aravis x264
pkg-config --modversion aravis-0.10 x264
go mod download
npm ci --prefix apps/web
```

Si la distribution fournit Aravis 0.8, mettre Aravis à niveau vers 0.10 avant de compiler l’option `aravis` ; les signatures C diffèrent. Le mode simulation et l’encodage x264 peuvent être utilisés indépendamment d’Aravis.

Deux terminaux, sans caméra :

```bash
go run -tags x264 ./apps/backend -simulate 2
npm run dev --prefix apps/web
```

Ouvrir <http://127.0.0.1:3000>. Cliquer sur **Voir en direct** pour établir WebRTC. Les aperçus JPEG à 1 FPS permettent aussi de travailler sans encodeur natif :

```bash
CGO_ENABLED=0 go run ./apps/backend -simulate 2
```

Dans ce dernier mode, l’API WebRTC retourne explicitement une erreur 503. Elle ne remplace jamais silencieusement la vidéo H.264 par une simulation.

Pour lancer les deux serveurs ensemble en mode matériel (site sur le port **3000**, API sur **8080**) :

```bash
./scripts/dev-real.sh
```

Cette commande compile Aravis/x264 et ne passe jamais `-simulate`. `Ctrl+C` arrête les deux serveurs. Les ports doivent être libres.

Avec les caméras réelles, sur le réseau GigE Vision :

```bash
go run -tags 'aravis x264' ./apps/backend
```

Aucune caméra, adresse IP ou référence de développement n’est codée en dur. La découverte utilise les annonces Aravis et identifie les caméras par numéro de série. Le réseau de la machine doit joindre directement le sous-réseau des caméras ; vérifier pare-feu et trafic UDP GigE Vision. L’acquisition démarre automatiquement pour les caméras accessibles. Le pilote sélectionne **Mono8** ; les formats disponibles sont exposés, mais les conversions Bayer, RGB et Mono10/12 restent à implémenter.

## Compilation de production

```bash
mkdir -p bin
go build -tags 'aravis x264' -o bin/veltryx-vision ./apps/backend
npm run build --prefix apps/web
./bin/veltryx-vision -listen 127.0.0.1:8080 -preset veryfast -bitrate 2500
# Dans un autre terminal :
npm run start --prefix apps/web
```

`-preset` accepte ultrafast, superfast, veryfast, faster, fast, medium et slow. Le tune est `zerolatency`, distinct du preset. Le débit est exprimé en kbit/s par caméra. `-config` et `-screenshots` changent les chemins persistants. Le défaut est `local-data/config.json` et `local-data/screenshots/`. Le processus journalise en JSON et libère acquisitions, buffers C, encodeurs et connexions lors d’un SIGINT/SIGTERM.

`BACKEND_URL=http://127.0.0.1:8080` règle la destination de la réécriture Next.js (à définir lors du build). La vidéo utilise une connexion WebRTC directe au backend ; le proxy HTTP transporte les offres SDP et le SSE. ICE utilise actuellement les interfaces locales, sans serveur STUN/TURN configurable : le déploiement livré vise un réseau local. Une exposition distante nécessite une configuration ICE, HTTPS, authentification et contrôle d’accès, non fournis ici. Le site écoute sur `0.0.0.0:3000`, accessible depuis les autres appareils via `http://<IP-locale-de-la-machine>:3000`. Le lanceur affiche les adresses locales au démarrage. Le backend HTTP reste sur `127.0.0.1:8080` ; son API et le SSE sont accessibles par le proxy du site.

## Fonctionnement

- Découverte toutes les trois secondes ; déconnexions conservées dans la liste, acquisition sans images valides arrêtée après trois secondes, tentative de réouverture aux scans suivants. Un arrêt demandé par l’utilisateur reste respecté après reconnexion, jusqu’au redémarrage du processus.
- Une acquisition et un Frame Hub par caméra. Images Gray8 immuables et détenues par Go, copiées avant restitution du buffer Aravis. Chaque consommateur dispose d’une file bornée qui abandonne l’image la plus ancienne.
- Un encodeur CPU à la demande par caméra, partagé entre au maximum 16 clients WebRTC. L’encodage continue jusqu’à l’arrêt du flux, la reconfiguration ou la déconnexion. Un IDR est produit environ toutes les deux secondes.
- ROI rectangulaires ou polygonales, normalisées ; écart de luminance, nombre minimum de pixels, confirmation, réarmement sur images calmes, cooldown et filtre des changements globaux. Les ROI invalides et auto-intersectées sont rejetées.
- Prébuffer limité à 60 images et 64 Mio par caméra ; file d’écriture limitée à 64 tâches et 64 Mio. Les captures avant passage peuvent être moins nombreuses si le budget mémoire ou les images disponibles le limitent. Sous charge, les abandons sont explicites via les statistiques ou les événements `error` ; l’acquisition ne bloque pas sur les consommateurs.
- Les nouvelles captures JPEG utilisent une qualité de 60/100 pour réduire le poids disque, à résolution originale. Le PNG reste sans perte. Les nouvelles captures sont aussi tournées avant encodage selon l’orientation choisie avec ↻ : 0°, 90°, 180° ou 270°. Cette préférence est persistée par caméra côté serveur et s’applique aux captures manuelles et automatiques. Les anciens fichiers sont conservés. Les timestamps des captures sont affichés avec trois chiffres de millisecondes sur le site.
- Les JPEG/PNG conservent les métadonnées dans un fichier JSON associé : série, passage, zone, séquence, horodatage UTC de réception et timestamp matériel non interprété. Les images d’un passage partagent un identifiant. Le backend recharge cet historique au démarrage.
- L’éditeur permet rectangles à deux coins, polygones à fermer, déplacement des poignées, Alt + clic pour ajouter un sommet et suppression du sommet sélectionné. Rotation 0/90/180/270° et redimensionnement conservent les coordonnées de l’image source. Le mode test affiche jusqu’à 2 048 pixels modifiés par ROI, à 10 Hz maximum. Il teste les ROI **enregistrées** côté Go.
- OCR : moteur injectable, état désactivé explicite, agrégation pondérée de plusieurs lectures d’un même passage. Aucun modèle, aucune reconnaissance fictive. L’ordonnancement automatique d’un moteur sera à connecter lorsqu’un moteur sera choisi.

## Vérifications

```bash
go test ./...
go test -race ./...
go test -race -tags 'aravis x264' ./...
go vet -tags 'aravis x264' ./...
npm run typecheck --prefix apps/web
npm run build --prefix apps/web
```

Le test WebRTC natif ouvre de vraies sockets UDP locales et reçoit du H.264 avec deux pairs Pion, depuis des images synthétiques. Les tests couvrent aussi le buffer, les consommateurs lents, les ROI, les seuils et le réarmement, la persistance et l’API avec deux caméras simulées. La compilation CGO et ces tests ne valident **pas** une caméra physique ni le décodage dans un navigateur.

À valider sur le matériel : découverte sur le bon NIC, `DeviceUserID`, plages GenICam réelles, contraintes d’incréments et d’exposition, stabilité des timestamps, débit GigE, pertes, débranchement/rebranchement, FPS à pleine résolution, latence et décodage H.264 dans les navigateurs ciblés. La métrique `latency_ms` mesure uniquement le retard côté Go depuis la réception horodatée ; elle ne mesure pas la latence caméra-écran. Les timestamps de caméras différentes ne sont pas synchronisés par PTP dans cette version.

Il n’y a pas encore de politique de rétention disque ni de pagination par curseur : l’historique API est limité à 100 images par défaut (maximum 1 000), son index est chargé en mémoire. Prévoir la rétention avant un fonctionnement continu. Les captures automatiques suivent les images reçues par le worker ; une surcharge peut créer des trous. Les détections ne constituent pas un chronométrage officiel.

Voir [architecture](docs/architecture.md) et [API](docs/api.md).

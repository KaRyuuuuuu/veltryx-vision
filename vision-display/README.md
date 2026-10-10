# Veltryx Vision Display (prototype)

Sortie vidéo indépendante de YOLO/OCR avec bandeau permanent sur l'image.
GStreamer + Cairo est utilisé aussi bien en prévisualisation graphique (`autovideosink`) qu'en affichage direct DRM/KMS (`kmssink`) **sans X11 ni Wayland**.

## Prérequis Arch

```bash
sudo pacman -S --needed python python-gobject python-cairo gst-python \
  gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad gst-plugins-ugly gst-libav
```

Ne pas exécuter avec le Python du venv YOLO : employer `/usr/bin/python` pour accéder à gi/GStreamer fournis par pacman.

## Tester sur Dragonfly (avec bureau Hyprland)

```bash
cd vision-display
/usr/bin/python display.py --mode window --source test
```

Pour utiliser ton tunnel RTSP (déjà actif sur Dragonfly) :

```bash
/usr/bin/python display.py --mode window --source rtsp://127.0.0.1:8554/h264 --label "FINISH"
```

Un bandeau horodaté `HH:MM:SS` et le label restent affichés. Pour un événement :

```bash
curl -X POST http://127.0.0.1:8095/event -H 'Content-Type: application/json' \
  -d '{"type":"PIT IN","car":"230","flag":"GREEN"}'
```

L'API n'est accessible que depuis la machine locale. Un événement ne modifie pas le chrono officiel.

## Mode headless DRM/KMS (à tester sur une machine de développement dédiée)

```bash
ls -l /dev/dri/
# Depuis un TTY sans environnement de bureau utilisant la carte :
/usr/bin/python display.py --mode kms --source rtsp://127.0.0.1:8554/h264 --label "FINISH"
```

Le mode KMS nécessite un périphérique DRM, les droits d'accès, un connecteur HDMI/DP actif et un compositeur graphique qui **ne détient pas** la sortie en exclusivité. Ne pas lancer `kmssink` en parallèle de Hyprland sur la même sortie. Le support et les performances varient selon le GPU. Option `--kms-bus-id` non fournie pour le moment : sélection du périphérique par GStreamer.

**Résolution :** le programme ne recadre pas la vidéo. Il préserve le rapport d'aspect ; l'affichage final peut ajouter des bandes noires. Le flux actuel `/h264` est 960×480 et non une vraie sortie 1080p. Le mode KMS est un prototype non validé sur ris-server-1.

## API locale

`POST /event` accepte du JSON avec `type`, `car` et `flag` (chaînes, 40 caractères max) ; `GET /state` renvoie l'état. Les événements sont temporaires, gardés en mémoire. Le bandeau affiche l'heure locale de l'hôte ; pour le chrono officiel, l'heure et l'horodatage d'acquisition devront être synchronisés.

Ce module est volontairement indépendant de ris-core et n'est pas activé automatiquement. Aucune installation systemd sur le serveur de production n'est effectuée.

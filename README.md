# Veltryx Vision

Prototype de reconnaissance de numéros de voitures, développé sur **Dragonfly**. **Ne pas déployer sur ris-server-1** avant validation.

## Architecture
- `backend/` : API Go, stockage local des jobs et lancement du worker.
- `vision/` : YOLO (détection véhicule) + Tesseract (OCR numérique), exécuté dans un processus Python isolé.
- `web/` : interface Next.js.

## Prérequis (Arch)
```bash
sudo pacman -S --needed go nodejs npm python python-pip ffmpeg tesseract tesseract-data-eng
```

## Installation
```bash
git clone git@github.com:KaRyuuuuuu/veltryx-vision.git
cd veltryx-vision
python -m venv .venv
.venv/bin/python -m pip install -r vision/requirements.txt
.venv/bin/python -c 'import cv2, ultralytics, pytesseract; print("Vision dependencies OK")'
cd web && npm install && cd ..
```
Attention : au premier lancement YOLO télécharge un modèle pré-entraîné et peut consommer plusieurs centaines de Mo. Le modèle général détecte les voitures **mais pas spécifiquement les plaques de numéro de course** : l'OCR est expérimental et nécessitera des images annotées pour devenir fiable.

## Démarrage en développement
Terminal 1 :
```bash
cd backend
VISION_PYTHON="$(realpath ../.venv/bin/python)" go run .
```
Terminal 2 :
```bash
cd web
npm run dev
```
Ouvrir http://localhost:3000 et sélectionner un fichier vidéo **déjà présent sur Dragonfly** avec son chemin absolu. Le serveur Next.js communique avec l'API Go via des requêtes locales; l'API accepte uniquement les fichiers sous `VISION_MEDIA_ROOT` (par défaut `~/Videos`).

### Enregistrer un clip de test depuis la caméra
Ouvrir au préalable le tunnel SSH RTSP habituel **sur Dragonfly** puis :
```bash
mkdir -p ~/Videos/veltryx-vision
ffmpeg -rtsp_transport tcp -i rtsp://127.0.0.1:8554/h264 -t 15 -c:v copy ~/Videos/veltryx-vision/basler.mkv
```
Aucune charge de décodage ou d'IA n'est lancée sur ris-server-1.

## Limites de la V1
OCR sur les voitures entières, faux positifs possibles. Pas de suivi multi-image, pas d'horodatage synchronisé à la ligne, pas de connexion à ris-core. Les événements sont uniquement des **candidats OCR** à valider manuellement; jamais des chronos officiels.

## Aperçu RTSP temps réel (Dragonfly uniquement)

Le navigateur affiche un MJPEG généré localement avec rectangles YOLO et OCR candidat. La caméra **n'est pas modifiée**. L'aperçu fonctionne par snapshots annotés (~6 FPS maximum), pas à 60 FPS, et peut subir une latence importante sur CPU.

1. Depuis Dragonfly, démarre un tunnel SSH RTSP : `ssh -N -L 8554:192.168.101.90:554 karyuu@192.168.103.49` (utilise l'adresse SSH joignable de ris-server-1).
2. Démarre le backend Go avec `VISION_PYTHON="$(realpath ../.venv/bin/python)" go run .` depuis `backend/`.
3. Démarre Next.js depuis `web/` avec `npm run dev`, puis clique sur **Démarrer le direct** sur http://localhost:3000.

La source RTSP est volontairement figée sur `rtsp://127.0.0.1:8554/h264` par défaut. Option de lancement du backend : `VISION_LIVE_SOURCE=rtsp://127.0.0.1:8554/h264`. Seules les sources loopback sont acceptées pour éviter de scanner le réseau de production. **Une seule fenêtre de visualisation recommandée** : chaque connexion démarre son propre processus d'inférence. Pour quitter, clique sur « Arrêter le direct ». Ce prototype n'est pas un outil de chronométrage officiel.

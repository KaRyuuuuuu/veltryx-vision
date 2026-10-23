# API REST V1

Base : `/api/v1`. JSON UTF-8 ; erreurs `{ "error": "message" }`. Corps limités à 1 Mio, champs inconnus et documents JSON multiples rejetés. Pas de CORS ouvert : Next.js relaie les requêtes sur la même origine. Les endpoints sont destinés à un réseau local de confiance.

| Méthode | Chemin | Description |
| --- | --- | --- |
| GET | `/health` | État HTTP |
| GET | `/cameras` | Série, ID Aravis, DeviceUserID, modèle, IP, formats, état, paramètres et plages disponibles |
| POST | `/discovery` | Scan immédiat puis liste |
| GET | `/cameras/{serial}` | Détail d’une caméra accessible |
| PUT | `/cameras/{serial}/settings` | Exposition, gain, dimensions et FPS ; redémarre l’acquisition si nécessaire |
| POST | `/cameras/{serial}/stream/start` | Démarrer l’acquisition |
| POST | `/cameras/{serial}/stream/stop` | Arrêter acquisition et WebRTC |
| POST | `/cameras/{serial}/webrtc` | Offre SDP `{type:"offer",sdp:"..."}` → réponse SDP |
| GET | `/cameras/{serial}/stats` | Statistiques acquisition et encodage |
| GET | `/cameras/{serial}/frame` | Dernière image originale en JPEG ; aperçu et éditeur |
| GET / PUT | `/cameras/{serial}/orientation` | Orientation partagée pour affichage et fichiers capturés : `{ "rotation": 90 }`, valeurs 0/90/180/270 ; persistance par caméra |
| GET / PUT | `/cameras/{serial}/roi` | Tableau de ROI par série |
| POST | `/cameras/{serial}/motion-test` | `{ "enabled": true }` active les pixels de diagnostic SSE |
| POST | `/cameras/{serial}/screenshots` | `{ "format": "jpeg" }` ou `png` → capture créée, HTTP 201 |
| GET | `/passages?serial=...&limit=100` | Passages complets (max. 200), une capture représentative et le nombre total d’images ; déclenchement prioritaire, image centrale pour les archives |
| GET | `/screenshots?serial=...&limit=100` | Captures récentes, maximum 1 000 ; regrouper par `passage` |
| GET | `/screenshots/{id}/image` | Image stockée |
| GET | `/events` | SSE : `data:` contient type, serial, time et data |

La résolution est décrite par ses plages largeur/hauteur et incréments GenICam, pas une liste de tailles arbitraires. Le détail dépend des capacités effectivement lisibles du périphérique. `serial` doit être encodé dans l’URL.

Exemple réglages (la caméra vérifie aussi ses propres limites) :

```json
{"width":640,"height":360,"fps":25,"exposure_us":10000,"gain":0}
```

Exemple `PUT /cameras/SIM-001/roi` :

```json
[
  {
    "id":"timing-line",
    "name":"Ligne de passage",
    "kind":"polygon",
    "points":[{"x":0.2,"y":0.3},{"x":0.4,"y":0.3},{"x":0.6,"y":0.9},{"x":0.3,"y":0.9}],
    "pixel_delta":30,
    "min_pixels":150,
    "confirm_frames":2,
    "quiet_frames":3,
    "cooldown_ms":1000,
    "max_changed_ratio":0.8,
    "pre_frames":3,
    "post_frames":3
  }
]
```

`kind` : `rectangle` (quatre sommets consécutifs, alignés sur les axes de l’image source) ou `polygon` (3 à 64 sommets sans intersection). Maximum 16 ROI par caméra. Les coordonnées sont dans `[0,1]`. Les valeurs pré/post sont limitées à 60, avec un budget mémoire supplémentaire sur le prébuffer. Une confirmation utilise les images consécutives effectivement reçues par le détecteur. Les paramètres doivent être enregistrés avant le test.

Événements : `cameras`, `settings`, `motion`, `passage`, `screenshot`, `error`. Un événement `passage` signale le déclenchement, pas la fin des écritures ; les images arrivent ensuite via `screenshot`. Les SSE n’ont pas de rejeu durable et peuvent perdre des événements sur un client lent. Recharger `/screenshots` à la reconnexion. `motion-test` est un réglage partagé par caméra, pas par navigateur.

`capture.dropped` cumule les abandons des files des abonnés du hub ; `capture.acquisition_dropped` estime les trous des identifiants de frames lorsque le périphérique les fournit. `capture.measured_fps` mesure les frames reçues par Go, indépendamment du FPS configuré. `capture.latency_ms` est un délai local depuis l’horodatage de réception Go, pas la latence du réseau GigE ni de l’affichage. `video.bitrate_kbps` mesure les octets H.264 produits avant RTP/SRTP, pas le débit réseau total. L’aperçu JPEG n’entre pas dans les statistiques H.264.

Statuts : 400 validation, 404 caméra/capture absente, 409 aucune image ou acquisition arrêtée, 422 refus de réglages matériels/négociation, 503 encodeur non disponible, 500 persistance/erreur interne. Après modification de résolution ou reconnexion, renégocier WebRTC.

Les paramètres acceptent aussi `auto_exposure`, `auto_gain`, `fps_unlimited` (booléens). Les modes automatiques utilisent GenICam Continuous ; FPS libres désactive le limiteur matériel. Les valeurs manuelles associées sont ignorées quand le mode automatique correspondant est actif. `capabilities` expose les booléens de prise en charge. Les anciens clients qui omettent ces champs restent en mode manuel.

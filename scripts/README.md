# Lancement matériel

```bash
bash scripts/dev-real.sh
# Depuis le dossier scripts :
bash dev-real.sh
```

Le script reste actif pendant l’exécution des deux serveurs. Ctrl+C les arrête.
Il affiche les étapes de démarrage et les erreurs de dépendances, de compilation ou d’écoute réseau.

Si le démarrage échoue avant la compilation :

```bash
pkg-config --print-errors --exists aravis-0.10 x264
pkg-config --modversion aravis-0.10 x264
```

Ces deux modules pkg-config sont requis avec leurs en-têtes de développement. Le pilote actuel ne cible pas les signatures Aravis 0.8. Pour une installation dans un préfixe personnalisé, configurer `PKG_CONFIG_PATH`.

Aucun tunnel vidéo, FFmpeg ou GStreamer n’est utilisé. Les commandes d’installation sont documentées dans le README racine.

# Déploiement

Compiler le binaire Go avec Aravis 0.10/x264, puis construire Next.js. Déployer sur une machine ayant accès au réseau des caméras ; exposer la signalisation et le SSE via la réécriture Next.js. Les médias utilisent les ports UDP ICE directs. Les variables, commandes, limites réseau et chemins persistants sont détaillés dans le README. L’authentification, HTTPS et TURN doivent être ajoutés avant une exposition hors réseau de confiance.

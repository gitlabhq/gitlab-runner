---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Mise à l'échelle automatique de GitLab CI sur AWS Fargate"
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!warning]
> Le pilote Fargate est pris en charge par la communauté. Le support GitLab essaiera d'aider à déboguer les problèmes, mais n'offre aucune garantie.

Le pilote [exécuteur personnalisé](../../executors/custom.md) GitLab pour [AWS Fargate](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate) lance automatiquement un conteneur sur l'Amazon Elastic Container Service (ECS) pour exécuter chaque job GitLab CI.

Une fois que vous avez effectué les tâches décrites dans ce document, l'exécuteur peut exécuter des jobs initiés depuis GitLab. Chaque fois qu'un commit est effectué dans GitLab, l'instance GitLab notifie le runner qu'un nouveau job est disponible. Le runner démarre ensuite une nouvelle tâche dans le cluster ECS cible, en se basant sur une définition de tâche que vous avez configurée dans AWS ECS. Vous pouvez configurer une définition de tâche AWS ECS pour utiliser n'importe quelle image Docker. Avec cette approche, vous disposez d'une flexibilité totale dans le type de builds que vous pouvez exécuter sur AWS Fargate.

![Architecture du pilote GitLab Runner Fargate](../img/runner_fargate_driver_ssh.png)

Ce document présente un exemple destiné à vous donner une compréhension initiale de l'implémentation. Il n'est pas destiné à un usage en production ; une sécurité supplémentaire est requise dans AWS.

Par exemple, vous pourriez avoir besoin de deux groupes de sécurité AWS :

- Un utilisé par l'instance EC2 qui héberge GitLab Runner et qui accepte uniquement les connexions SSH provenant d'une plage d'adresses IP externes restreinte (pour l'accès administratif).
- Un qui s'applique aux tâches Fargate et qui autorise le trafic SSH uniquement depuis l'instance EC2.

Pour tout registre de conteneurs non public, votre tâche ECS nécessite soit des [autorisations IAM (pour AWS ECR uniquement)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html), soit une [authentification auprès d'un registre privé pour les tâches](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html) pour les registres privés non-ECR.

Vous pouvez utiliser CloudFormation ou Terraform pour automatiser le provisionnement et la configuration de votre infrastructure AWS.

Les jobs CI/CD utilisent l'image définie dans la tâche ECS, plutôt que la valeur du mot-clé `image:` dans votre fichier `.gitlab-ci.yml`. ECS ne vous permet pas de remplacer l'image utilisée pour une tâche ECS.

Pour contourner cette limitation, vous pouvez :

- Créer et utiliser une image dans la définition de tâche ECS qui contient toutes les dépendances de build de tous les projets pour lesquels le runner est utilisé.
- Créer plusieurs définitions de tâches ECS avec différentes images et spécifier l'ARN dans la variable CI/CD `FARGATE_TASK_DEFINITION`.
- Envisager d'utiliser l'exécuteur Kubernetes sur un cluster Amazon EKS créé avec [AWS EKS Blueprints](https://aws-ia.github.io/terraform-aws-eks-blueprints/). Le pilote d'exécuteur personnalisé Fargate n'est pas maintenu par GitLab et est pris en charge dans la limite du possible.

Pour plus d'informations, consultez [Get started with GitLab EKS Fargate runners in 1 hour and zero code](https://about.gitlab.com/blog/eks-fargate-runner/).

> [!warning]
> Fargate abstrait les hôtes de conteneurs, ce qui limite la configurabilité des propriétés des hôtes de conteneurs. Cela affecte les charges de travail du runner qui nécessitent des E/S élevées vers le disque ou le réseau, car ces propriétés ont une configurabilité limitée ou nulle avec Fargate. Avant d'utiliser GitLab Runner sur Fargate, assurez-vous que les charges de travail du runner avec des caractéristiques de calcul élevées en termes de CPU, de mémoire, d'E/S disque ou d'E/S réseau sont adaptées à Fargate.

## Prérequis {#prerequisites}

Avant de commencer, vous devez avoir :

- Un utilisateur AWS IAM disposant des autorisations pour créer et configurer des ressources EC2, ECS et ECR.
- Un VPC AWS et des sous-réseaux.
- Un ou plusieurs groupes de sécurité AWS.

## Étape 1 : Préparer une image de conteneur pour la tâche AWS Fargate {#step-1-prepare-a-container-image-for-the-aws-fargate-task}

Préparez une image de conteneur. Vous pouvez téléverser cette image dans un registre, où elle peut être utilisée pour créer des conteneurs lors de l'exécution des jobs GitLab.

1. Assurez-vous que l'image dispose des outils nécessaires pour construire votre job CI. Par exemple, un projet Java nécessite un `Java JDK` et des outils de build comme Maven ou Gradle. Un projet Node.js nécessite `node` et `npm`.
1. Assurez-vous que l'image dispose de GitLab Runner, qui gère les artefacts et la mise en cache. Consultez la section [Run](../../executors/custom.md#run) (étape d'exécution) de la documentation de l'exécuteur personnalisé pour obtenir des informations supplémentaires.
1. Assurez-vous que l'image du conteneur peut accepter une connexion SSH par authentification par clé publique. Le runner utilise cette connexion pour envoyer les commandes de build définies dans le fichier `.gitlab-ci.yml` au conteneur sur AWS Fargate. Les clés SSH sont automatiquement gérées par le pilote Fargate. Le conteneur doit être en mesure d'accepter les clés provenant de la variable d'environnement `SSH_PUBLIC_KEY`.

Consultez un [exemple Debian](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian) qui inclut GitLab Runner et la configuration SSH. Consultez un [exemple Node.js](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate).

## Étape 2 : Pousser l'image de conteneur vers un registre {#step-2-push-the-container-image-to-a-registry}

Après avoir créé votre image, publiez-la dans un registre de conteneurs pour l'utiliser dans la définition de tâche ECS.

- Pour créer un dépôt et pousser une image vers ECR, suivez la documentation [Amazon ECR Repositories](https://docs.aws.amazon.com/AmazonECR/latest/userguide/Repositories.html).
- Pour utiliser l'AWS CLI afin de pousser une image vers ECR, suivez la documentation [Getting Started with Amazon ECR using the AWS CLI](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html).
- Pour utiliser le [GitLab Container Registry](https://docs.gitlab.com/user/packages/container_registry/) , vous pouvez utiliser l'exemple [Debian](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian) ou [NodeJS](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate). L'image Debian est publiée dans `registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`. L'image d'exemple NodeJS est publiée dans `registry.gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate:latest`.

## Étape 3 : Créer une instance EC2 pour GitLab Runner {#step-3-create-an-ec2-instance-for-gitlab-runner}

Créez maintenant une instance AWS EC2. À l'étape suivante, vous installerez GitLab Runner dessus.

1. Accédez à <https://console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard>.
1. Pour l'instance, sélectionnez l'AMI Ubuntu Server 18.04 LTS. Le nom peut être différent selon la région AWS que vous avez sélectionnée.
1. Pour le type d'instance, choisissez t2.micro. Sélectionnez **Suivant : Configurer les détails de l'instance**.
1. Conservez la valeur par défaut pour **Nombre d'instances**.
1. Pour **Réseau**, sélectionnez votre VPC.
1. Définissez **Attribuer automatiquement une adresse IP publique** sur **Activer**.
1. Sous **Rôle IAM**, sélectionnez **Créer un nouveau rôle IAM**. Ce rôle est uniquement à des fins de test et n'est pas sécurisé.
   1. Sélectionnez **Créer un rôle**.
   1. Choisissez **Service AWS** et sous **Cas d'utilisation courants**, sélectionnez **EC2**. Puis sélectionnez **Suivant : Autorisations**.
   1. Cochez la case pour la politique **AmazonECS_FullAccess**. Sélectionnez **Suivant : Tags**.
   1. Sélectionnez **Suivant : Vérifier**.
   1. Saisissez un nom pour le rôle IAM, par exemple `fargate-test-instance`, et sélectionnez **Créer un rôle**.
1. Revenez à l'onglet du navigateur où vous créez l'instance.
1. À gauche de **Créer un nouveau rôle IAM**, sélectionnez le bouton d'actualisation. Choisissez le rôle `fargate-test-instance`. Sélectionnez **Suivant : Ajouter de l'espace de stockage**.
1. Sélectionnez **Suivant : Ajouter des tags**.
1. Sélectionnez **Suivant : Configurer le groupe de sécurité**.
1. Sélectionnez **Créer un nouveau groupe de sécurité**, nommez-le `fargate-test`, et assurez-vous qu'une règle pour SSH est définie (`Type: SSH, Protocol: TCP, Port Range: 22`). Vous devez spécifier les plages d'adresses IP pour les règles entrantes et sortantes.
1. Sélectionnez **Vérifier et lancer**.
1. Sélectionnez **Lancer**.
1. Facultatif. Sélectionnez **Créer une nouvelle paire de clés**, nommez-la `fargate-runner-manager` et sélectionnez **Télécharger la paire de clés**. La clé privée pour SSH est téléchargée sur votre ordinateur (vérifiez le répertoire configuré dans votre navigateur).
1. Sélectionnez **Lancer des instances**.
1. Sélectionnez **Afficher les instances**.
1. Attendez que l'instance soit opérationnelle. Notez l'adresse `IPv4 Public IP`.

## Étape 4 : Installer et configurer GitLab Runner sur l'instance EC2 {#step-4-install-and-configure-gitlab-runner-on-the-ec2-instance}

Installez maintenant GitLab Runner sur l'instance Ubuntu.

1. Accédez aux **Paramètres > CI/CD** de votre projet GitLab et développez la section Runners. Sous **Configurer manuellement un Runner spécifique**, notez le jeton d'enregistrement.
1. Assurez-vous que votre fichier de clé dispose des autorisations correctes en exécutant `chmod 400 path/to/downloaded/key/file`.
1. Connectez-vous en SSH à l'instance EC2 que vous avez créée en utilisant :

   ```shell
   ssh ubuntu@[ip_address] -i path/to/downloaded/key/file
   ```

1. Une fois connecté avec succès, exécutez les commandes suivantes :

   ```shell
   sudo mkdir -p /opt/gitlab-runner/{metadata,builds,cache}
   curl -s "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash
   sudo apt install gitlab-runner
   ```

1. Exécutez cette commande avec l'URL GitLab et le jeton d'enregistrement que vous avez notés à l'étape 1.

   ```shell
   sudo gitlab-runner register --url "https://gitlab.com/" --registration-token TOKEN_HERE --name fargate-test-runner --run-untagged --executor custom -n
   ```

1. Exécutez `sudo vim /etc/gitlab-runner/config.toml` et ajoutez le contenu suivant :

   ```toml
   concurrent = 1
   check_interval = 0

   [session_server]
     session_timeout = 1800

   [[runners]]
     name = "fargate-test"
     url = "https://gitlab.com/"
     token = "__REDACTED__"
     executor = "custom"
     builds_dir = "/opt/gitlab-runner/builds"
     cache_dir = "/opt/gitlab-runner/cache"
     [runners.custom]
       volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
       config_exec = "/opt/gitlab-runner/fargate"
       config_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "config"]
       prepare_exec = "/opt/gitlab-runner/fargate"
       prepare_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "prepare"]
       run_exec = "/opt/gitlab-runner/fargate"
       run_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "run"]
       cleanup_exec = "/opt/gitlab-runner/fargate"
       cleanup_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "cleanup"]
   ```

1. Si vous disposez d'une instance GitLab Self-Managed avec une CA privée, ajoutez cette ligne :

   ```toml
          volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
   ```

   [En savoir plus sur l'approbation du certificat](../tls-self-signed.md#trusting-the-certificate-for-the-other-cicd-stages).

   La section du fichier `config.toml` affichée ci-dessous est créée par la commande d'enregistrement. Ne la modifiez pas.

   ```toml
   concurrent = 1
   check_interval = 0

   [session_server]
     session_timeout = 1800

   name = "fargate-test"
   url = "https://gitlab.com/"
   token = "__REDACTED__"
   executor = "custom"
   ```

1. Exécutez `sudo vim /etc/gitlab-runner/fargate.toml` et ajoutez le contenu suivant :

   ```toml
   LogLevel = "info"
   LogFormat = "text"

   [Fargate]
     Cluster = "test-cluster"
     Region = "us-east-2"
     Subnet = "subnet-xxxxxx"
     SecurityGroup = "sg-xxxxxxxxxxxxx"
     TaskDefinition = "test-task:1"
     EnablePublicIP = true

   [TaskMetadata]
     Directory = "/opt/gitlab-runner/metadata"

   [SSH]
     Username = "root"
     Port = 22
   ```

   - Notez la valeur de `Cluster` et le nom de `TaskDefinition`. Cet exemple montre `test-task` avec `:1` comme numéro de révision. Si aucun numéro de révision n'est spécifié, la dernière révision **active** est utilisée.
   - Choisissez votre région. Prenez la valeur `Subnet` à partir de l'instance du gestionnaire de runner.
   - Pour trouver l'ID du groupe de sécurité :

     1. Dans AWS, dans la liste des instances, sélectionnez l'instance EC2 que vous avez créée. Les détails sont affichés.
     1. Sous **Groupes de sécurité**, sélectionnez le nom du groupe que vous avez créé.
     1. Copiez l’**ID du groupe de sécurité**.

     Dans un environnement de production, suivez les [directives AWS](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-security-groups.html) pour la configuration et l'utilisation des groupes de sécurité.

   - Si `EnablePublicIP` est défini sur true, l'adresse IP publique du conteneur de tâche est collectée pour établir la connexion SSH.
   - Si `EnablePublicIP` est défini sur false :
     - Le pilote Fargate utilise l'adresse IP privée du conteneur de tâche. Pour établir une connexion lorsque la valeur est `false`, le groupe de sécurité VPC doit avoir une règle entrante pour le port 22 (SSH), où la source est le CIDR du VPC.
     - Pour récupérer des dépendances externes, les conteneurs AWS Fargate provisionnés doivent avoir accès à l'Internet public. Pour fournir un accès à l'Internet public pour les conteneurs AWS Fargate, vous pouvez utiliser une passerelle NAT dans le VPC.

   - Le numéro de port du serveur SSH est facultatif. S'il est omis, le port SSH par défaut (22) est utilisé.
   - Pour plus d'informations sur les paramètres de la section, consultez la [documentation du pilote Fargate](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/-/tree/master/docs#configuration).

1. Installez le pilote Fargate :

   ```shell
   sudo curl -Lo /opt/gitlab-runner/fargate "https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/latest/fargate-linux-amd64"
   sudo chmod +x /opt/gitlab-runner/fargate
   ```

## Étape 5 : Créer un cluster ECS Fargate {#step-5-create-an-ecs-fargate-cluster}

Un cluster Amazon ECS est un regroupement d'instances de conteneurs ECS.

1. Accédez à [`https://console.aws.amazon.com/ecs/home#/clusters`](https://console.aws.amazon.com/ecs/home#/clusters).
1. Sélectionnez **Créer un cluster**.
1. Choisissez le type **Réseau uniquement**. Sélectionnez **Étape suivante**.
1. Nommez-le `test-cluster` (identique à celui dans `fargate.toml`).
1. Sélectionnez **Créer**.
1. Sélectionnez **Afficher le cluster**. Notez les parties région et ID de compte de la valeur `Cluster ARN`.
1. Sélectionnez **Mettre à jour le cluster**.
1. À côté de `Default capacity provider strategy`, sélectionnez **Ajouter un autre fournisseur** et choisissez `FARGATE`. Sélectionnez **Mise à jour**.

Consultez la [documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html) AWS pour des instructions détaillées sur la configuration et l'utilisation d'un cluster sur ECS Fargate.

## Étape 6 : Créer une définition de tâche ECS {#step-6-create-an-ecs-task-definition}

Dans cette étape, vous allez créer une définition de tâche de type `Fargate` et référencer l'image de conteneur que vous pourriez utiliser pour vos builds CI.

1. Accédez à [`https://console.aws.amazon.com/ecs/home#/taskDefinitions`](https://console.aws.amazon.com/ecs/home#/taskDefinitions).
1. Sélectionnez **Créer une nouvelle définition de tâche**.
1. Choisissez **FARGATE** et sélectionnez **Étape suivante**.
1. Nommez-la `test-task`. (Remarque : le nom est la même valeur définie dans le fichier `fargate.toml` mais sans `:1`).
1. Sélectionnez des valeurs pour **Mémoire de la tâche (Go)** et **CPU de la tâche (vCPU)**.
1. Sélectionnez **Ajouter un conteneur**. Ensuite :
   1. Nommez-le `ci-coordinator`, afin que le pilote Fargate puisse injecter la variable d'environnement `SSH_PUBLIC_KEY`.
   1. Définissez l'image (par exemple `registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`).
   1. Définissez le mappage de port pour 22/TCP.
   1. Sélectionnez **Ajouter**.
1. Sélectionnez **Créer**.
1. Sélectionnez **Afficher la définition de la tâche**.

> [!warning]
> Une seule tâche Fargate peut lancer un ou plusieurs conteneurs. Le pilote Fargate injecte la variable d'environnement `SSH_PUBLIC_KEY` uniquement dans les conteneurs portant le nom `ci-coordinator`. Vous devez avoir un conteneur portant ce nom dans toutes les définitions de tâches utilisées par le pilote Fargate. Le conteneur portant ce nom doit être celui qui dispose du serveur SSH et de toutes les exigences de GitLab Runner installées, comme décrit ci-dessus.

Consultez la [documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/create-task-definition.html) AWS pour des instructions détaillées sur la configuration et l'utilisation des définitions de tâches.

Pour des informations sur les autorisations du service ECS requises pour lancer des images depuis un AWS ECR, consultez [Amazon ECS task execution IAM role](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html).

Pour des informations sur l'authentification ECS auprès des registres privés, y compris ceux hébergés sur une instance GitLab, consultez [Private registry authentication for tasks](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html).

À ce stade, le gestionnaire de runner et le pilote Fargate sont configurés et prêts à commencer à exécuter des jobs sur AWS Fargate.

## Étape 7 : Tester la configuration {#step-7-test-the-configuration}

Votre configuration devrait maintenant être prête à l'emploi.

1. Dans votre projet GitLab, créez un fichier `.gitlab-ci.yml` :

   ```yaml
   test:
     script:
       - echo "It works!"
       - for i in $(seq 1 30); do echo "."; sleep 1; done
   ```

1. Accédez à **CI/CD > Pipelines** de votre projet.
1. Sélectionnez **Exécuter le pipeline**.
1. Mettez à jour la branche et toutes les variables, puis sélectionnez **Exécuter le pipeline**.

> [!note]
> Les mots-clés `image` et `service` dans votre fichier `.gitlab-ci.yml` sont ignorés. Le runner utilise uniquement les valeurs spécifiées dans la définition de tâche.

## Nettoyage {#clean-up}

Si vous souhaitez effectuer un nettoyage après avoir testé l'exécuteur personnalisé avec AWS Fargate, supprimez les objets suivants :

- Instance EC2, paire de clés, rôle IAM et groupe de sécurité créés à l'[étape 3](#step-3-create-an-ec2-instance-for-gitlab-runner).
- Cluster ECS Fargate créé à l'[étape 5](#step-5-create-an-ecs-fargate-cluster).
- Définition de tâche ECS créée à l'[étape 6](#step-6-create-an-ecs-task-definition).

## Configurer une tâche AWS Fargate privée {#configure-a-private-aws-fargate-task}

Pour garantir un niveau de sécurité élevé, configurez [une tâche AWS Fargate privée](https://repost.aws/knowledge-center/ecs-fargate-tasks-private-subnet). Dans cette configuration, les exécuteurs utilisent uniquement des adresses IP AWS internes. Ils n'autorisent que le trafic sortant depuis AWS afin que les jobs CI/CD s'exécutent sur une instance AWS Fargate privée.

Pour configurer une tâche AWS Fargate privée, effectuez les étapes suivantes pour configurer AWS et exécuter la tâche AWS Fargate dans le sous-réseau privé :

1. Assurez-vous que le sous-réseau public existant n'a pas réservé toutes les adresses IP dans la plage d'adresses du VPC. Inspectez les plages d'adresses `cidr` du VPC et du sous-réseau. Si la plage d'adresses `cidr` du sous-réseau est un sous-ensemble de la plage d'adresses `cidr` du VPC, ignorez les étapes 2 et 4. Sinon, votre VPC ne dispose d'aucune plage d'adresses libre, vous devez donc supprimer et recréer le VPC et le sous-réseau public :
   1. Supprimez votre sous-réseau et VPC existants.
   1. [Créez un VPC](https://docs.aws.amazon.com/vpc/latest/privatelink/create-interface-endpoint.html#create-interface-endpoint) avec la même configuration que le VPC que vous avez supprimé et mettez à jour l'adresse `cidr`, par exemple `10.0.0.0/23`.
   1. [Créez un sous-réseau public](https://docs.aws.amazon.com/vpc/latest/privatelink/interface-endpoints.html) avec la même configuration que le sous-réseau que vous avez supprimé. Utilisez une adresse `cidr` qui est un sous-ensemble de la plage d'adresses du VPC, par exemple `10.0.0.0/24`.
1. [Créez un sous-réseau privé](https://docs.aws.amazon.com/vpc/latest/userguide/create-subnet.html#create-subnets) avec la même configuration que le sous-réseau public. Utilisez une plage d'adresses `cidr` qui ne chevauche pas la plage du sous-réseau public, par exemple `10.0.1.0/24`.
1. [Créez une passerelle NAT](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html) et placez-la dans le sous-réseau public.
1. Modifiez la table de routage du sous-réseau privé afin que la destination `0.0.0.0/0` pointe vers la passerelle NAT.
1. Mettez à jour la configuration `farget.toml` :

   ```toml
   Subnet = "private-subnet-id"
   EnablePublicIP = false
   UsePublicIP = false
   ```

1. Ajoutez la politique inline suivante au rôle IAM associé à votre tâche Fargate (le rôle IAM associé aux tâches Fargate est généralement nommé `ecsTaskExecutionRole` et devrait déjà exister.)

   ```json
   {
       "Statement": [
           {
               "Sid": "VisualEditor0",
               "Effect": "Allow",
               "Action": [
                   "secretsmanager:GetSecretValue",
                   "kms:Decrypt",
                   "ssm:GetParameters"
               ],
               "Resource": [
                   "arn:aws:secretsmanager:*:<account-id>:secret:*",
                   "arn:aws:kms:*:<account-id>:key/*"
               ]
           }
       ]
   }
   ```

1. Modifiez les « règles entrantes » de votre groupe de sécurité pour référencer le groupe de sécurité lui-même. Dans la fenêtre de configuration AWS :
   - Définissez `Type` sur `ssh`.
   - Définissez `Source` sur `Custom`.
   - Sélectionnez le groupe de sécurité.
   - Supprimez la règle entrante existante qui autorise l'accès SSH depuis n'importe quel hôte.

> [!warning]
> Lorsque vous supprimez la règle entrante existante, vous ne pouvez plus utiliser SSH pour vous connecter à l'instance Amazon Elastic Compute Cloud.

Pour plus d'informations, consultez la documentation AWS suivante :

- [Amazon ECS task execution IAM role](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)
- [Amazon ECR interface VPC endpoints (AWS PrivateLink)](https://docs.aws.amazon.com/AmazonECR/latest/userguide/vpc-endpoints.html)
- [Amazon ECS interface VPC endpoints](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/vpc-endpoints.html)
- [VPC with public and private subnets](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-example-private-subnets-nat.html)

## Dépannage {#troubleshooting}

### Erreur `No Container Instances were found in your cluster` lors du test de la configuration {#no-container-instances-were-found-in-your-cluster-error-when-testing-the-configuration}

`error="starting new Fargate task: running new task on Fargate: error starting AWS Fargate Task: InvalidParameterException: No Container Instances were found in your cluster."`

Le pilote AWS Fargate nécessite que le cluster ECS soit configuré avec une [stratégie de fournisseur de capacité par défaut](#step-5-create-an-ecs-fargate-cluster).

Pour aller plus loin :

- Une [stratégie de fournisseur de capacité](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cluster-capacity-providers.html) par défaut est associée à chaque cluster Amazon ECS. Si aucune autre stratégie de fournisseur de capacité ou type de lancement n'est spécifié, le cluster utilise cette stratégie lorsqu'une tâche s'exécute ou qu'un service est créé.
- Si un [`capacityProviderStrategy`](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html#ECS-RunTask-request-capacityProviderStrategy) est spécifié, le paramètre `launchType` doit être omis. Si aucun `capacityProviderStrategy` ou `launchType` n'est spécifié, le `defaultCapacityProviderStrategy` du cluster est utilisé.

### Erreur de métadonnées `file does not exist` lors de l'exécution des jobs {#metadata-file-does-not-exist-error-when-running-jobs}

`Application execution failed PID=xxxxx error="obtaining information about the running task: trying to access file \"/opt/gitlab-runner/metadata/<runner_token>-xxxxx.json\": file does not exist" cleanup_std=err job=xxxxx project=xx runner=<runner_token>`

Assurez-vous que votre politique de rôle IAM est correctement configurée et peut effectuer des opérations d'écriture pour créer le fichier JSON de métadonnées dans `/opt/gitlab-runner/metadata/`. Pour tester dans un environnement hors production, utilisez la politique AmazonECS_FullAccess. Vérifiez votre politique de rôle IAM selon les exigences de sécurité de votre organisation.

### `connection timed out` lors de l'exécution des jobs {#connection-timed-out-when-running-jobs}

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"172.x.x.x\": connecting to server: connecting to server \"172.x.x.x:22\" as user \"root\": dial tcp 172.x.x.x:22: connect: connection timed out"`

Si `EnablePublicIP` est configuré sur false, assurez-vous que votre groupe de sécurité VPC dispose d'une règle entrante autorisant la connectivité SSH. Votre conteneur de tâche AWS Fargate doit accepter le trafic SSH provenant de l'instance EC2 GitLab Runner.

### `connection refused` lors de l'exécution des jobs {#connection-refused-when-running-jobs}

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"10.x.x.x\": connecting to server: connecting to server \"10.x.x.x:22\" as user \"root\": dial tcp 10.x.x.x:22: connect: connection refused"`

Assurez-vous que le conteneur de tâche a le port 22 exposé et que le mappage de port est configuré conformément aux instructions de [Étape 6 : Créer une définition de tâche ECS](#step-6-create-an-ecs-task-definition). Si le port est exposé et que le conteneur est configuré :

1. Vérifiez s'il y a des erreurs pour le conteneur dans **Amazon ECS > Clusters > Choisissez votre définition de tâche > Tâches**.
1. Affichez les tâches avec le statut `Stopped` et vérifiez la dernière qui a échoué. L'onglet **journaux** contient plus de détails en cas d'échec d'un conteneur.

Vous pouvez également vous assurer que vous pouvez exécuter le conteneur Docker localement.

### Erreur : `ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain` {#error-ssh-unable-to-authenticate-attempted-methods-none-publickey-no-supported-methods-remain}

L'erreur suivante se produit si un type de clé non pris en charge est utilisé en raison d'une version plus ancienne du pilote AWS Fargate.

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"172.x.x.x\": connecting to server: connecting to server \"172.x.x.x:22\" as user \"root\": ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain"`

Pour résoudre ce problème, installez le dernier pilote AWS Fargate sur l'instance EC2 GitLab Runner :

```shell
sudo curl -Lo /opt/gitlab-runner/fargate "https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/latest/fargate-linux-amd64"
sudo chmod +x /opt/gitlab-runner/fargate
```

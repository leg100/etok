# Github App

This guide walks you through deploying the Github app.

1. First ensure you have [installed the operator]({{< ref "docs/tutorials/getting_started.md" >}}).

2. Run the `github deploy` command, specifying a URL that will receive the webhook events:

    ```bash
    etok github deploy --url [WEBHOOK_URL]
    ```

3. A browser window opens:

    ![create-1](/create-github-app.png)

4. Click the button to be forwarded to Github:

    ![create-2](/create-github-app-2.png)

5. Enter a unique name for the app and click the button to create the app. You'll then be redirected to install the app on your Github account:

    ![create-3](/create-github-app-3.png)

6. Select the repositories you want to give access to your app, and click 'Install'. The app will be installed and you'll be redirected to a page confirming a successful installation if all went well:

    ![create-4](/create-github-app-4.png)

7. Return to your terminal and you should find the app has been deployed:

    ```bash
    Your browser has been opened to visit: http://localhost:43963/github-app/setup
    Successfully created github app "my-etok-app". App ID: 111060
    Persisted credentials to secret github/creds
    Github app successfully installed. Installation ID: 16341142
    Created resource ClusterRole webhook
    Created resource ClusterRoleBinding webhook
    Created resource Deployment github/webhook
    Created resource Service github/webhook
    Created resource ServiceAccount github/webhook
    Waiting for Deployment to be ready
    ```

8. Before you can receive webhook events you need to install an ingress resource. The contents of the resource will depend upon your setup, which ingress controller you're using, etc. Regardless of your setup, you need to ensure the namespace matches the namespace the webhook is deployed into (default is `github`), and the service name and port are correct (defaults are `webhook` and `9001`). The host should match the URL you specified in the deployment too. For example, here is an ingress resource for the `nginx` ingress controller:

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: Ingress
      name: webhook
      namespace: github
    spec:
      ingressClassName: nginx
      rules:
      - host: webhook.etok.dev
        http:
          paths:
          - backend:
              service:
                name: webhook
                port:
                  number: 9001
            path: /
            pathType: Prefix
      tls:
      - hosts:
        - webhook.etok.dev
        secretName: webhook-etok-dev-tls
    ```

    You may also need the ingress resource to reference a secret containing a valid SSL certificate and key. In the example a secret named `webhook-etok-dev-tls` contains the certificate and key for `webhook.etok.dev`. You may want to deploy [cert-manager](https://cert-manager.io/), which automatically creates such secrets on your behalf.

    Once you've crafted your ingress resource, deploy it:

    ```bash
    kubectl apply -f ingress.yaml
    ```

    Once its been assigned an IP address it's been successfully setup:

    ```bash
    > kubectl -n github get ingress
    NAME      CLASS   HOSTS              ADDRESS        PORTS     AGE
    webhook   nginx   webhook.etok.dev   35.214.23.46   80, 443   18s
    ```

9. Now you can test the app. You'll need to have a github repo cloned to your local machine (and make sure you gave the app permission to access the repo).

    Within the directory of the repo, create an etok workspace:

    ```bash
    cd repo
    etok workspace new dev
    ```

    Create a new branch, write some terraform configuration, and commit and push it:

    ```bash
    git checkout -b app-test
    ```

    ```bash
    cat > main.tf <<EOF
    resource "null_resource" "hello" {}
    EOF
    ```

    ```bash
    git add main.tf
    git commit -m test
    git push -u origin app-test
    ```

    Click on the link to open a new pull request:

    ```bash
    Enumerating objects: 4, done.
    Counting objects: 100% (4/4), done.
    Writing objects: 100% (3/3), 268 bytes | 268.00 KiB/s, done.
    Total 3 (delta 0), reused 0 (delta 0)
    remote: 
    remote: Create a pull request for 'test' on GitHub by visiting:
    remote:      https://github.com/leg100/etok-e2e/pull/new/test
    remote: 
    To github.com:leg100/etok-e2e.git
     * [new branch]      test -> test
    Branch 'test' set up to track remote branch 'test' from 'origin'.

    ```

    Click on the `Checks` tab:

    ![create-5](/create-github-app-5.png)

    You can see that a terraform plan has been triggered for the workspace. The symbols `+1/~0/−0` indicate that the plan adds one new resource, with no changes and no deletions.

    To apply the plan, click `Apply`, triggering a new run:

    ![create-6](/create-github-app-6.png)

1. You've now confirmed the Github app is deployed and functioning. It'll continue to trigger runs whenever a commit is pushed.

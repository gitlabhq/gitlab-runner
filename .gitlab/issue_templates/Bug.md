## Summary

<!--
Briefly summarize the bug
-->

## Steps to reproduce

<!--
What do you need to do to reproduce the bug? Please include job definitions or git repository structure if relevant
-->

<!--
Please add the definition of the job from `.gitlab-ci.yml` that is failing
inside of the code blocks (```) below.
-->
<details>
<summary> .gitlab-ci.yml </summary>

```yml
Add the job definition that is failing here
```
</details>

## Actual behavior

<!--
What actually happens
-->

## Expected behavior

<!--
What you should see instead
-->

## Relevant logs and/or screenshots

<!--
Paste the job logs inside of the code blocks (```) below so it would be
easier to read.
-->

<details>
<summary> job log </summary>

```sh
Add the job log
```
</details>

## Environment description

<!--
Are you using shared Runners on GitLab.com? Or is it a custom installation?
Which executors are used? Please also provide the versions of related tools
like `docker info` if you are using the Docker executor.
-->

<!--
Please add the contents of `config.toml` inside of the code blocks (```)
below, remember to remove any secret tokens!
-->
<details>
<summary> config.toml contents </summary>

```toml
Add your configuration here
```
</details>

### Used GitLab Runner version

<!--
Please run and paste the output of `gitlab-runner --version`. If you are using
a Runner where you don't have access to, please paste at least the first lines
the from build log, like:

```
Running with gitlab-ci-multi-runner 1.4.2 (bcc1794)
Using Docker executor with image golang:1.8 ...
```
-->

## Possible fixes

<!--
(If you can, link to the line of code that might be responsible for the problem)
--->

/label ~bug

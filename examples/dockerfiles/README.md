Dockerfiles in the wild and testdata to see if they build

## Django

```
go run . --org chainguard-private examples/dockerfiles/django.before.Dockerfile > examples/dockerfiles/django.after.Dockerfile

```
docker build -t django-local -f ./examples/dockerfiles/django.after.Dockerfile examples/dockerfiles/django/
```
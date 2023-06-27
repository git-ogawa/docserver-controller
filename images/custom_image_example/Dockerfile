From python:3.11-slim-bookworm

RUN pip install --no-cache-dir \
    mkdocs \
    mkdocs-material \
    plantuml-markdown

RUN mkdir /docs
WORKDIR /docs

ENTRYPOINT ["mkdocs"]
CMD ["serve", "--dev-addr=0.0.0.0:8000"]

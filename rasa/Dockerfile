FROM khalosa/rasa-aarch64:3.5.2

WORKDIR /app

USER root

RUN pip install spacy && python -m spacy download en_core_web_md

# Copy actions folder to working directory
COPY ./ /app

RUN rasa train

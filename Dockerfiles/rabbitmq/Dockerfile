FROM rabbitmq
WORKDIR /apps
COPY . .
ENTRYPOINT [ "sh" ]
RUN apt update && apt install wget python3 -y
RUN ["sh","./rabbitmq-setup.sh"]
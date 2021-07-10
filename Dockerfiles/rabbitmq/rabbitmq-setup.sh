#!/usr/bin/bash
(/usr/local/bin/docker-entrypoint.sh rabbitmq-server)&
sleep 5
#login with guestuser
rabbitmqctl add_user mqadmin mqadminpassword
rabbitmqctl set_user_tags mqadmin administrator
rabbitmqctl set_permissions -p / mqadmin ".*" ".*" ".*"
rabbitmq-plugins enable rabbitmq_jms_topic_exchange
rabbitmqctl authenticate_user  mqadmin mqadminpassword

rabbitmq-plugins enable rabbitmq_management
wget http://127.0.0.1:15672/cli/rabbitmqadmin
chmod +x rabbitmqadmin
cp rabbitmqadmin /etc/rabbitmq
cp rabbitmqadmin /usr/local/bin/

#create a queue and exchange
rabbitmqadmin declare exchange name=test-exchange type=fanout
rabbitmqadmin declare queue name=test-queue
rabbitmqadmin declare binding source=test-exchange destination=test-queue routing_key=sendq
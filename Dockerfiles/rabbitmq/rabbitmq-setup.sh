#!/usr/bin/bash
(./usr/loacal/bin/docker-entrypoint.sh)&
sleep 5
#login with guestuser
rabbitmqctl add_user mqadmin mqadminpassword
rabbitmqctl set_user_tags mqadmin administrator
rabbitmqctl set_permissions -p / mqadmin ".*" ".*" ".*"
rabbitmq-plugins enable rabbitmq_jms_topic_exchange
rabbitmqctl authenticate_user  mqadmin mqadminpassword

sudo rabbitmq-plugins enable rabbitmq_management
wget http://127.0.0.1:15672/cli/rabbitmqadmin
chmod +x rabbitmqadmin
mv rabbitmqadmin /etc/rabbitmq


#create a queue and exchange
rabbitmqadmin declare exchange name=test-exchange type=fanout
rabbitmqadmin declare queue name=test-queue
rabbitmqadmin declare binding source=test-exchange destination=test-queue destination-type=queue routing_key=sendq
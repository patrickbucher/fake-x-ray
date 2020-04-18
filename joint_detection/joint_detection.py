#!/usr/bin/env python3

import json
import os
import sys
import time

import pika


def main():
    amqp_uri = os.environ.get('AMQP_URI', 'localhost')
    max_retries = 10
    retry_after_seconds = 2
    retries = 0
    connection = None
    while retries < max_retries:
        try:
            connection = pika.BlockingConnection(
                pika.ConnectionParameters(amqp_uri))
            break
        except pika.exceptions.AMQPConnectionError as e:
            print(e)
            retries += 1
            print(f'connection to {amqp_uri} failed, waiting...')
            time.sleep(retry_after_seconds)

    if not connection:
        print(f'unable to connect to {amqp_uri} after {retries} retries')
        sys.exit(1)
    print(f'connected to to {amqp_uri}')

    channel = connection.channel()
    channel.queue_declare(queue='joint_detection')
    channel.basic_qos(prefetch_count=1)
    channel.basic_consume(
        queue='joint_detection',
        on_message_callback=callback)
    channel.start_consuming()


def callback(ch, method, props, body):
    print('task', body)
    json_payload = body.decode('utf-8')
    payload = json.loads(json_payload)
    joints = detect_joints(payload)
    print('joints', joints)
    for joint in joints:
        ch.basic_publish(
            exchange='',
            routing_key='joints',
            properties=pika.BasicProperties(
                correlation_id=props.correlation_id),
            body=joint)
    ch.basic_ack(delivery_tag=method.delivery_tag)


def detect_joints(body):
    lower = body['raw'].find('(')
    upper = body['raw'].find(')')
    if lower == -1 or upper == -1 or lower >= upper:
        return ''
    segments = body['raw'][lower+1:upper].split(',')
    segments = [s.strip() for s in segments]
    joints = []
    detected_joints = []
    print('joint names', body['joint_names'])
    for s in segments:
        for joint in body['joint_names']:
            if s.startswith(joint):
                joints.append(s)
                detected_joints.append(joint)
                break
    undetected_joints = set(body['joint_names']) - set(detected_joints)
    for undetected_joint in undetected_joints:
        joints.append(f'{undetected_joint}=-1')
    return joints


if __name__ == '__main__':
    main()

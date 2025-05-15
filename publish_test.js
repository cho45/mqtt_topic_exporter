// publish_test.js
// 簡単なMQTTパブリッシュスクリプト (Node.js)
const mqtt = require('mqtt');

const broker = 'mqtt://localhost:1883';
const topic = '/test/topic';
const value = 123.45;

const client = mqtt.connect(broker);

client.on('connect', function () {
  console.log(`Publishing ${value} to ${topic}`);
  client.publish(topic, value.toString(), {}, function () {
    client.end();
  });
});

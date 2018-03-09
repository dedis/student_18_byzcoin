const protobuf = require('protobufjs');
const fs = require('fs');

const root = new protobuf.Root();
root.define('cothority');

const regex = /^.*\.proto$/;

fs.readdir('models', (err, items) => {
  items.forEach(file => {
    if (regex.test(file)) {
      root.loadSync('models/' + file);
    }
  });

  fs.writeFileSync('models/skeleton.js', `export default '${JSON.stringify(root.toJSON())}';`)
});


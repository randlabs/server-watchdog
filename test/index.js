const process = require('process');
const path = require('path');
const { spawn } = require('child_process');
const fetch = require('node-fetch');
const colors = require('colors');
const fkill = require('fkill');

//------------------------------------------------------------------------------

let settings;

//------------------------------------------------------------------------------

main().then(() => {
	console.log("Done!");
	process.exit(0);
}).catch((err) => {
	process.stdout.write(colors.brightRed('ERROR') + "\n");
	if (err.stack) {
		console.log(err.stack);
	}
	else {
		console.log(err.toString());
	}
	process.exit(1);
});

async function main() {
	let childProc;

	process.stdout.write("Initializing... ");
	await loadSettings();
	process.stdout.write(colors.brightGreen('OK') + "\n");

	//--------

	process.stdout.write("Sending an error message thru channel '" + settings.channel + "'... ");
	await sendRequest('/notify', {
		channel: settings.channel,
		message: 'This is a sample error message from the Server Watchdog test application',
		severity: 'error'
	});
	process.stdout.write(colors.brightGreen('OK') + "\n");

	//--------

	process.stdout.write("Sending a warning message thru channel '" + settings.channel + "'... ");
	await sendRequest('/notify', {
		channel: settings.channel,
		message: 'This is a sample warning message from the Server Watchdog test application',
		severity: 'warn'
	});
	process.stdout.write(colors.brightGreen('OK') + "\n");

	//--------

	process.stdout.write("Sending an information message thru channel '" + settings.channel + "'... ");
	await sendRequest('/notify', {
		channel: settings.channel,
		message: 'This is a sample information message from the Server Watchdog test application',
		severity: 'info'
	});
	process.stdout.write(colors.brightGreen('OK') + "\n");

	//--------

	process.stdout.write("Launching a child NodeJS process... ");
	childProc = spawnNodeJs();
	process.stdout.write(colors.brightGreen('OK') + "\n");

	process.stdout.write("Sending watch process for process #" + childProc.pid.toString() + "... ");
	await sendRequest('/process/watch', {
		channel: settings.channel,
		pid: childProc.pid,
		name: 'Child NodeJS exit 0',
		severity: 'error'
	});
	process.stdout.write(colors.brightGreen('OK') + "\n");

	process.stdout.write("Exiting gracefully from child process... ");
	childProc.stdin.write('process.exit(0);\n');
	childProc.stdin.end();
	process.stdout.write(colors.brightGreen('OK') + "\n");

	//--------

	process.stdout.write("Launching again a child NodeJS process... ");
	childProc = spawnNodeJs();
	process.stdout.write(colors.brightGreen('OK') + "\n");

	process.stdout.write("Sending watch process for process #" + childProc.pid.toString() + "... ");
	await sendRequest('/process/watch', {
		channel: settings.channel,
		pid: childProc.pid,
		name: 'Child NodeJS exit 1',
		severity: 'error'
	});
	process.stdout.write(colors.brightGreen('OK') + "\n");

	process.stdout.write("Exiting from child process with exit code different than 0... ");
	childProc.stdin.write('process.exit(1);\n');
	childProc.stdin.end();
	process.stdout.write(colors.brightGreen('OK') + "\n");

	//--------

	process.stdout.write("Launching a third child NodeJS process... ");
	childProc = spawnNodeJs();
	process.stdout.write(colors.brightGreen('OK') + "\n");

	process.stdout.write("Sending watch process for process #" + childProc.pid.toString() + "... ");
	await sendRequest('/process/watch', {
		channel: settings.channel,
		pid: childProc.pid,
		name: 'Child NodeJS exit 1',
		severity: 'error'
	});
	process.stdout.write(colors.brightGreen('OK') + "\n");

	process.stdout.write("Killing the child process with exit code different than 0... ");
	await fkill(childProc.pid, {
		force: true
	});
	process.stdout.write(colors.brightGreen('OK') + "\n");
}

function sleep(ms) {
	return new Promise(resolve => setTimeout(resolve, ms));
}

async function loadSettings() {
	let filename = 'settings.json';
	let st;

	for (let idx = 0; idx < process.argv.length; idx++) {
		if (process.argv[idx] == '--settings') {
			if (idx + 1 >= process.argv.length) {
				throw new Error("Missing filename in '--settings' parameter.");
			}
			filename = process.argv[idx + 1];
		}
	}
	try {
		filename = path.resolve(__dirname, filename);
		// eslint-disable-next-line global-require
		st = require(filename);
	}
	catch (err) {
		throw new Error("Unable to load settings file.");
	}

	if (typeof st.server !== 'object' && (!Array.isArray(st.server))) {
		throw new Error("Missing or invalid server configuration in settings file.");
	}
	if (typeof st.server.port !== 'number' || st.server.port < 1 || st.server.port > 65535 || (!Number.isInteger(st.server.port))) {
		throw new Error("Missing or invalid server port in settings file.");
	}
	if (typeof st.server.apiKey !== 'string' || st.server.apiKey.length == 0) {
		throw new Error("Missing or invalid server API key in settings file.");
	}

	if (typeof st.channels !== 'object' && (!Array.isArray(st.channels))) {
		throw new Error("Missing or invalid log channels in settings file.");
	}
	let channelNames = Object.keys(st.channels);
	if (channelNames.length == 0) {
		throw new Error("Missing or invalid log channels in settings file.");
	}

	settings = {
		server: {
			port: st.server.port,
			apiKey: st.server.apiKey
		},
		channel: channelNames[0]
	}
}

async function sendRequest(service, postData, hasResponse) {
	let queryOpts = {
		method: postData ? 'POST' : 'GET',
		headers: {
			'Content-Type': 'application/json',
			'X-Api-Key': settings.server.apiKey
		},
		...(postData) && { body: JSON.stringify(postData) }
	};

	let response = await fetch('http://127.0.0.1:' + settings.server.port.toString() + service, queryOpts);
	if (response.status != 200) {
		await throwUnexpectedStatusCode(response);
	}
	if (hasResponse) {
		const json = await response.json();
		return json;
	}
	return;
}

async function throwUnexpectedStatusCode(response) {
	let errMsg = "Unexpected status " + response.status.toString();

	try {
		let msg = await response.text();
		if (msg.length > 0) {
			errMsg += ' [' + msg + ']';
		}
	}
	catch (err) {
		//keep ESLint happy
	}
	throw new Error(errMsg);
}

function spawnNodeJs() {
	const ls = spawn(process.execPath, []);

	ls.stdout.on('data', (data) => {
		//keep ESLint happy
		const s = String.fromCharCode.apply(null, new Uint8Array(data));
	});

	ls.stderr.on('data', (data) => {
		//keep ESLint happy
		const s = String.fromCharCode.apply(null, new Uint8Array(data));
	});

	ls.on('close', (code) => {
		//keep ESLint happy
	});

	ls.stdin.setEncoding('utf-8');

	return ls;
}

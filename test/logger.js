const log4js = require('log4js');
const cluster = require('cluster');
const process = require('process');

//------------------------------------------------------------------------------

let logger = null;

//------------------------------------------------------------------------------

function initialize() {
	if (cluster.isMaster) {
		log4js.configure({
			appenders: {
				out: {
					type: 'console',
					layout: {
						type: 'pattern',
						pattern: '[%d{yyyy-MM-dd hh:mm:ss}] [%[%p%]] - %m'
					},
					level: 'all'
				}
			},
			categories: {
				default: {
					appenders: [ 'out' ],
					level: 'all'
				}
			}
		});
		logger = log4js.getLogger();
	}
}

function error(message) {
	if (cluster.isMaster) {
		logger.error(message);
	}
	else {
		process.send({ log: 'error', message });
	}
}

function warn(message) {
	if (cluster.isMaster) {
		logger.warn(message);
	}
	else {
		process.send({ log: 'warn', message });
	}
}

function info(message) {
	if (cluster.isMaster) {
		logger.info(message);
	}
	else {
		process.send({ log: 'info', message });
	}
}

function debug(message) {
	if (cluster.isMaster) {
		logger.debug(message);
	}
	else {
		process.send({ log: 'debug', message });
	}
}

function onWorkerEvent(pid, event) {
	if (event.log === 'error') {
		error("(#" + pid.toString() + ") " + event.message);
		return true;
	}
	if (event.log === 'warn') {
		warn("(#" + pid.toString() + ") " + event.message);
		return true;
	}
	if (event.log === 'info') {
		info("(#" + pid.toString() + ") " + event.message);
		return true;
	}
	if (event.log === 'debug') {
		debug("(#" + pid.toString() + ") " + event.message);
		return true;
	}
	return false;
}

module.exports = {
	initialize,
	error,
	warn,
	info,
	debug,
	onWorkerEvent
};

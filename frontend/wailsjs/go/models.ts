export namespace config {
	
	export class Config {
	    limbus_folder: string;
	    current_version: string;
	    current_bot_version: string;
	    current_limbonia_version: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limbus_folder = source["limbus_folder"];
	        this.current_version = source["current_version"];
	        this.current_bot_version = source["current_bot_version"];
	        this.current_limbonia_version = source["current_limbonia_version"];
	    }
	}

}

export namespace updater {
	
	export class UpdateResponse {
	    limbo_version: string;
	    launcher_version: string;
	    bot_version: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limbo_version = source["limbo_version"];
	        this.launcher_version = source["launcher_version"];
	        this.bot_version = source["bot_version"];
	    }
	}

}


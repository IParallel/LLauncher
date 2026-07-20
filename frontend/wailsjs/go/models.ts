export namespace client {
	
	export class ChangelogEntry {
	    release_id: string;
	    version: string;
	    published_at: string;
	    title: string;
	    body_md: string;
	
	    static createFrom(source: any = {}) {
	        return new ChangelogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.release_id = source["release_id"];
	        this.version = source["version"];
	        this.published_at = source["published_at"];
	        this.title = source["title"];
	        this.body_md = source["body_md"];
	    }
	}

}

export namespace config {
	
	export class Config {
	    limbus_folder: string;
	    current_client_version: string;
	    current_limbonia_version?: string;
	    current_bot_version?: string;
	    current_version?: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limbus_folder = source["limbus_folder"];
	        this.current_client_version = source["current_client_version"];
	        this.current_limbonia_version = source["current_limbonia_version"];
	        this.current_bot_version = source["current_bot_version"];
	        this.current_version = source["current_version"];
	    }
	}

}

export namespace main {
	
	export class LauncherUpdate {
	    state: string;
	    version: string;
	    percent: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LauncherUpdate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.version = source["version"];
	        this.percent = source["percent"];
	        this.message = source["message"];
	    }
	}

}

export namespace updater {
	
	export class UpdateResponse {
	    client_version: string;
	    launcher_version: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.client_version = source["client_version"];
	        this.launcher_version = source["launcher_version"];
	    }
	}

}


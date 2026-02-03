type WebGLContext = WebGLRenderingContext | WebGL2RenderingContext;

type WebGL2QueryExt = {
  TIME_ELAPSED_EXT: number;
  GPU_DISJOINT_EXT: number;
  beginQueryEXT: (target: number, query: WebGLQuery) => void;
  endQueryEXT: (target: number) => void;
  getQueryObjectEXT: (
    query: WebGLQuery,
    pname: number,
  ) => number | boolean;
  QUERY_RESULT_AVAILABLE_EXT: number;
  QUERY_RESULT_EXT: number;
};

type WebGL1QueryExt = {
  TIME_ELAPSED_EXT: number;
  GPU_DISJOINT_EXT: number;
  createQueryEXT: () => WebGLQuery;
  deleteQueryEXT: (query: WebGLQuery) => void;
  beginQueryEXT: (target: number, query: WebGLQuery) => void;
  endQueryEXT: (target: number) => void;
  getQueryObjectEXT: (
    query: WebGLQuery,
    pname: number,
  ) => number | boolean;
  QUERY_RESULT_AVAILABLE_EXT: number;
  QUERY_RESULT_EXT: number;
};

export class GpuTimer {
  private ext: WebGL1QueryExt | WebGL2QueryExt | null = null;
  private activeQuery: WebGLQuery | null = null;
  private queue: WebGLQuery[] = [];
  lastMs: number | null = null;

  init(gl: WebGLContext) {
    if (this.ext) return;
    const ext =
      (gl.getExtension(
        "EXT_disjoint_timer_query_webgl2",
      ) as WebGL2QueryExt | null) ||
      (gl.getExtension("EXT_disjoint_timer_query") as WebGL1QueryExt | null);
    this.ext = ext;
  }

  begin(gl: WebGLContext) {
    if (!this.ext || this.activeQuery) return;
    const query = gl.createQuery ? gl.createQuery() : this.ext.createQueryEXT();
    if (!query) return;
    this.activeQuery = query;
    if (gl.beginQuery) {
      gl.beginQuery(this.ext.TIME_ELAPSED_EXT, query);
    } else {
      this.ext.beginQueryEXT(this.ext.TIME_ELAPSED_EXT, query);
    }
  }

  end(gl: WebGLContext) {
    if (!this.ext || !this.activeQuery) return;
    if (gl.endQuery) {
      gl.endQuery(this.ext.TIME_ELAPSED_EXT);
    } else {
      this.ext.endQueryEXT(this.ext.TIME_ELAPSED_EXT);
    }
    this.queue.push(this.activeQuery);
    this.activeQuery = null;
  }

  poll(gl: WebGLContext) {
    if (!this.ext || this.queue.length === 0) return;
    const query = this.queue[0];
    const available = gl.getQueryParameter
      ? (gl.getQueryParameter(query, gl.QUERY_RESULT_AVAILABLE) as boolean)
      : (this.ext.getQueryObjectEXT(
          query,
          this.ext.QUERY_RESULT_AVAILABLE_EXT,
        ) as boolean);
    const disjoint = gl.getParameter(this.ext.GPU_DISJOINT_EXT) as boolean;
    if (!available || disjoint) return;
    const result = gl.getQueryParameter
      ? (gl.getQueryParameter(query, gl.QUERY_RESULT) as number)
      : (this.ext.getQueryObjectEXT(
          query,
          this.ext.QUERY_RESULT_EXT,
        ) as number);
    this.queue.shift();
    if (gl.deleteQuery) {
      gl.deleteQuery(query);
    } else {
      this.ext.deleteQueryEXT(query);
    }
    this.lastMs = result / 1_000_000;
  }
}

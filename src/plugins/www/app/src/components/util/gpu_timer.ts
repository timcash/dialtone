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
    let query: WebGLQuery | null = null;
    if ("createQuery" in gl) {
      const gl2 = gl as WebGL2RenderingContext;
      query = gl2.createQuery();
      if (!query) return;
      this.activeQuery = query;
      gl2.beginQuery(this.ext.TIME_ELAPSED_EXT, query);
      return;
    }
    const ext1 = this.ext as WebGL1QueryExt;
    query = ext1.createQueryEXT();
    if (!query) return;
    this.activeQuery = query;
    ext1.beginQueryEXT(this.ext.TIME_ELAPSED_EXT, query);
  }

  end(gl: WebGLContext) {
    if (!this.ext || !this.activeQuery) return;
    if ("endQuery" in gl) {
      (gl as WebGL2RenderingContext).endQuery(this.ext.TIME_ELAPSED_EXT);
    } else {
      (this.ext as WebGL1QueryExt).endQueryEXT(this.ext.TIME_ELAPSED_EXT);
    }
    this.queue.push(this.activeQuery);
    this.activeQuery = null;
  }

  poll(gl: WebGLContext) {
    if (!this.ext || this.queue.length === 0) return;
    const query = this.queue[0];
    const available =
      "getQueryParameter" in gl
        ? ((gl as WebGL2RenderingContext).getQueryParameter(
            query,
            (gl as WebGL2RenderingContext).QUERY_RESULT_AVAILABLE,
          ) as boolean)
        : ((this.ext as WebGL1QueryExt).getQueryObjectEXT(
            query,
            this.ext.QUERY_RESULT_AVAILABLE_EXT,
          ) as boolean);
    const disjoint = gl.getParameter(this.ext.GPU_DISJOINT_EXT) as boolean;
    if (!available || disjoint) return;
    const result =
      "getQueryParameter" in gl
        ? ((gl as WebGL2RenderingContext).getQueryParameter(
            query,
            (gl as WebGL2RenderingContext).QUERY_RESULT,
          ) as number)
        : ((this.ext as WebGL1QueryExt).getQueryObjectEXT(
            query,
            this.ext.QUERY_RESULT_EXT,
          ) as number);
    this.queue.shift();
    if ("deleteQuery" in gl) {
      (gl as WebGL2RenderingContext).deleteQuery(query);
    } else {
      (this.ext as WebGL1QueryExt).deleteQueryEXT(query);
    }
    this.lastMs = result / 1_000_000;
  }
}

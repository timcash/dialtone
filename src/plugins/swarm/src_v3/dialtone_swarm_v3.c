#include <arpa/inet.h>
#include <errno.h>
#include <inttypes.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <uv.h>

#include "udx.h"

typedef struct app_s {
  uv_loop_t *loop;
  udx_t udx;
  udx_socket_t socket;
  udx_stream_t stream;

  uv_timer_t send_timer;
  uv_timer_t exit_timer;

  struct sockaddr_storage peer_addr;
  bool has_peer;
  bool no_send;

  uint32_t local_id;
  uint32_t peer_id;
  uint32_t sent_count;
  uint32_t max_count;
  uint64_t interval_ms;
  uint64_t exit_after_ms;
  char message[1024];
} app_t;

static void usage(const char *prog) {
  fprintf(stderr,
          "Usage:\n"
          "  %s --bind-port <port> [options]\n\n"
          "Options:\n"
          "  --bind-ip <ip>           Local bind IP (v4/v6, default: 0.0.0.0)\n"
          "  --bind-port <port>       Local UDP port (required)\n"
          "  --peer-ip <ip>           Remote peer IP (v4/v6)\n"
          "  --peer-port <port>       Remote peer UDP port\n"
          "  --local-id <id>          Local UDX stream ID (default: 1)\n"
          "  --peer-id <id>           Remote UDX stream ID (default: 2)\n"
          "  --message <text>         Message to send (default: hello)\n"
          "  --count <n>              Number of sends (default: 1)\n"
          "  --interval-ms <ms>       Delay between sends (default: 500)\n"
          "  --no-send                Connect/read only; do not send\n"
          "  --exit-after-ms <ms>     Stop loop after timeout (default: 0)\n"
          "  --help                   Show this help\n\n"
          "Examples:\n"
          "  # Receiver (connect/read only)\n"
          "  %s --bind-port 9002 --peer-ip 203.0.113.10 --peer-port 9001 "
          "--local-id 2 --peer-id 1 --no-send\n"
          "  # Sender\n"
          "  %s --bind-port 9001 --peer-ip 198.51.100.20 --peer-port 9002 "
          "--local-id 1 --peer-id 2 --message \"hello\"\n",
          prog, prog, prog);
}

static int parse_u32(const char *s, uint32_t *out) {
  char *end = NULL;
  unsigned long v = strtoul(s, &end, 10);
  if (s[0] == '\0' || end == NULL || *end != '\0' || v > UINT32_MAX) return -1;
  *out = (uint32_t) v;
  return 0;
}

static int parse_u64(const char *s, uint64_t *out) {
  char *end = NULL;
  unsigned long long v = strtoull(s, &end, 10);
  if (s[0] == '\0' || end == NULL || *end != '\0') return -1;
  *out = (uint64_t) v;
  return 0;
}

static int parse_sockaddr(const char *ip, uint32_t port, struct sockaddr_storage *out) {
  if (ip == NULL || out == NULL || port > 65535) return -1;

  struct sockaddr_in v4;
  if (uv_ip4_addr(ip, (int) port, &v4) == 0) {
    memset(out, 0, sizeof(*out));
    memcpy(out, &v4, sizeof(v4));
    return 0;
  }

  struct sockaddr_in6 v6;
  if (uv_ip6_addr(ip, (int) port, &v6) == 0) {
    memset(out, 0, sizeof(*out));
    memcpy(out, &v6, sizeof(v6));
    return 0;
  }

  return -1;
}

static void on_ack(udx_stream_write_t *req, int status, int unordered) {
  if (status < 0) {
    fprintf(stderr, "write ack error: %s\n", uv_strerror(status));
  }
  (void) unordered;
  free(req);
}

static void send_once(app_t *app) {
  uv_buf_t buf = uv_buf_init(app->message, (unsigned int) strlen(app->message));
  udx_stream_write_t *req = malloc(udx_stream_write_sizeof(1));
  if (req == NULL) {
    fprintf(stderr, "malloc failed for write request\n");
    return;
  }

  int rc = udx_stream_write(req, &app->stream, &buf, 1, on_ack);
  if (rc < 0) {
    fprintf(stderr, "udx_stream_write failed: %s\n", uv_strerror(rc));
    free(req);
    return;
  }

  app->sent_count++;
  printf("sent[%u/%u]: %s\n", app->sent_count, app->max_count, app->message);
  fflush(stdout);
}

static void on_send_tick(uv_timer_t *t) {
  app_t *app = t->data;
  if (app->sent_count >= app->max_count) {
    uv_timer_stop(&app->send_timer);
    return;
  }
  send_once(app);
}

static void on_read(udx_stream_t *stream, ssize_t nread, const uv_buf_t *buf) {
  (void) stream;
  if (nread < 0) {
    fprintf(stderr, "read error: %s\n", uv_strerror((int) nread));
    return;
  }
  if (nread > 0) {
    printf("received[%zd]: %.*s\n", nread, (int) nread, buf->base);
    fflush(stdout);
  }
}

static void on_exit_timer(uv_timer_t *t) {
  app_t *app = t->data;
  printf("exit timeout reached (%" PRIu64 " ms)\n", app->exit_after_ms);
  fflush(stdout);
  uv_stop(app->loop);
}

int main(int argc, char **argv) {
  const char *bind_ip = "0.0.0.0";
  const char *effective_bind_ip = bind_ip;
  uint32_t bind_port = 0;
  const char *peer_ip = NULL;
  uint32_t peer_port = 0;

  app_t app;
  memset(&app, 0, sizeof(app));
  app.local_id = 1;
  app.peer_id = 2;
  app.max_count = 1;
  app.interval_ms = 500;
  snprintf(app.message, sizeof(app.message), "hello");

  for (int i = 1; i < argc; i++) {
    const char *arg = argv[i];
    if (strcmp(arg, "--help") == 0) {
      usage(argv[0]);
      return 0;
    } else if (strcmp(arg, "--no-send") == 0) {
      app.no_send = true;
    } else if (strcmp(arg, "--bind-ip") == 0 && i + 1 < argc) {
      bind_ip = argv[++i];
    } else if (strcmp(arg, "--peer-ip") == 0 && i + 1 < argc) {
      peer_ip = argv[++i];
    } else if (strcmp(arg, "--message") == 0 && i + 1 < argc) {
      snprintf(app.message, sizeof(app.message), "%s", argv[++i]);
    } else if (strcmp(arg, "--bind-port") == 0 && i + 1 < argc) {
      if (parse_u32(argv[++i], &bind_port) != 0 || bind_port > 65535) {
        fprintf(stderr, "invalid --bind-port\n");
        return 1;
      }
    } else if (strcmp(arg, "--peer-port") == 0 && i + 1 < argc) {
      if (parse_u32(argv[++i], &peer_port) != 0 || peer_port > 65535) {
        fprintf(stderr, "invalid --peer-port\n");
        return 1;
      }
    } else if (strcmp(arg, "--local-id") == 0 && i + 1 < argc) {
      if (parse_u32(argv[++i], &app.local_id) != 0) {
        fprintf(stderr, "invalid --local-id\n");
        return 1;
      }
    } else if (strcmp(arg, "--peer-id") == 0 && i + 1 < argc) {
      if (parse_u32(argv[++i], &app.peer_id) != 0) {
        fprintf(stderr, "invalid --peer-id\n");
        return 1;
      }
    } else if (strcmp(arg, "--count") == 0 && i + 1 < argc) {
      if (parse_u32(argv[++i], &app.max_count) != 0) {
        fprintf(stderr, "invalid --count\n");
        return 1;
      }
    } else if (strcmp(arg, "--interval-ms") == 0 && i + 1 < argc) {
      if (parse_u64(argv[++i], &app.interval_ms) != 0) {
        fprintf(stderr, "invalid --interval-ms\n");
        return 1;
      }
    } else if (strcmp(arg, "--exit-after-ms") == 0 && i + 1 < argc) {
      if (parse_u64(argv[++i], &app.exit_after_ms) != 0) {
        fprintf(stderr, "invalid --exit-after-ms\n");
        return 1;
      }
    } else {
      fprintf(stderr, "unknown or incomplete arg: %s\n", arg);
      usage(argv[0]);
      return 1;
    }
  }

  if (bind_port == 0) {
    fprintf(stderr, "--bind-port is required\n");
    usage(argv[0]);
    return 1;
  }

  if (peer_ip != NULL || peer_port != 0) {
    if (peer_ip == NULL || peer_port == 0) {
      fprintf(stderr, "--peer-ip and --peer-port must be set together\n");
      return 1;
    }
    app.has_peer = true;
  }

  if (!app.no_send && !app.has_peer) {
    fprintf(stderr, "sending requires --peer-ip/--peer-port\n");
    return 1;
  }

  // If peer discovery returns IPv6 and user kept default IPv4 bind, switch to IPv6 bind.
  if (app.has_peer && strcmp(bind_ip, "0.0.0.0") == 0 && strchr(peer_ip, ':') != NULL) {
    effective_bind_ip = "::";
  }

  app.loop = uv_default_loop();

  int rc = udx_init(app.loop, &app.udx, NULL);
  if (rc != 0) {
    fprintf(stderr, "udx_init failed: %s\n", uv_strerror(rc));
    return 1;
  }

  rc = udx_socket_init(&app.udx, &app.socket, NULL);
  if (rc != 0) {
    fprintf(stderr, "udx_socket_init failed: %s\n", uv_strerror(rc));
    return 1;
  }

  struct sockaddr_storage bind_addr;
  if (parse_sockaddr(effective_bind_ip, bind_port, &bind_addr) != 0) {
    fprintf(stderr, "invalid bind address %s:%u\n", effective_bind_ip, bind_port);
    return 1;
  }

  rc = udx_socket_bind(&app.socket, (const struct sockaddr *) &bind_addr, 0);
  if (rc != 0) {
    fprintf(stderr, "udx_socket_bind failed: %s\n", uv_strerror(rc));
    return 1;
  }

  struct sockaddr_storage bound;
  int bound_len = sizeof(bound);
  rc = udx_socket_getsockname(&app.socket, (struct sockaddr *) &bound, &bound_len);
  if (rc == 0 && bound.ss_family == AF_INET) {
    char ip[INET_ADDRSTRLEN];
    struct sockaddr_in *sin = (struct sockaddr_in *) &bound;
    uv_ip4_name(sin, ip, sizeof(ip));
    printf("bound=%s:%d\n", ip, ntohs(sin->sin_port));
  } else if (rc == 0 && bound.ss_family == AF_INET6) {
    char ip[INET6_ADDRSTRLEN];
    struct sockaddr_in6 *sin6 = (struct sockaddr_in6 *) &bound;
    uv_ip6_name(sin6, ip, sizeof(ip));
    printf("bound=[%s]:%d\n", ip, ntohs(sin6->sin6_port));
  }

  rc = udx_stream_init(&app.udx, &app.stream, app.local_id, NULL, NULL);
  if (rc != 0) {
    fprintf(stderr, "udx_stream_init failed: %s\n", uv_strerror(rc));
    return 1;
  }
  udx_stream_read_start(&app.stream, on_read);

  if (app.has_peer) {
    if (parse_sockaddr(peer_ip, peer_port, &app.peer_addr) != 0) {
      fprintf(stderr, "invalid peer address %s:%u\n", peer_ip, peer_port);
      return 1;
    }

    rc = udx_stream_connect(&app.stream, &app.socket, app.peer_id,
                            (const struct sockaddr *) &app.peer_addr);
    if (rc != 0) {
      fprintf(stderr, "udx_stream_connect failed: %s\n", uv_strerror(rc));
      return 1;
    }
    printf("peer=%s:%u local_id=%u peer_id=%u\n", peer_ip, peer_port, app.local_id,
           app.peer_id);
  }

  if (!app.no_send && app.max_count > 0) {
    uv_timer_init(app.loop, &app.send_timer);
    app.send_timer.data = &app;
    uv_timer_start(&app.send_timer, on_send_tick, 200, app.interval_ms);
  }

  if (app.exit_after_ms > 0) {
    uv_timer_init(app.loop, &app.exit_timer);
    app.exit_timer.data = &app;
    uv_timer_start(&app.exit_timer, on_exit_timer, app.exit_after_ms, 0);
  }

  uv_run(app.loop, UV_RUN_DEFAULT);
  return 0;
}

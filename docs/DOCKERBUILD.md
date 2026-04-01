# Build and Test with Docker

### Omnigate (lmgate + Ollama, CPU only)

```bash
cp .env.sample .env   # edit with your values
docker compose -f docker/docker-compose.build.omni.yml up -d
```

### Standalone (External LLM Backend)

```bash
cp .env.sample .env   # edit with your values
docker compose -f docker/docker-compose.build.standalone.yml up -d
```

Set `LMGATE_UPSTREAM_URL` to point to your existing LLM service.

### Omnigate — NVIDIA GPU

All-in-one image bundling LM Gate and Ollama with NVIDIA GPU acceleration. Requires the [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).

```bash
cp .env.sample .env   # edit with your values
docker compose -f docker/docker-compose.build.omni.nvidia.yml up -d
```

### Omnigate — AMD GPU

All-in-one image bundling LM Gate and Ollama with AMD GPU acceleration via ROCm. Requires `/dev/kfd` and `/dev/dri` device access on the host. AMD APU users (including Ryzen AI 9 series) should use this variant as well.

```bash
cp .env.sample .env   # edit with your values
docker compose -f docker/docker-compose.build.omni.amd.yml up -d
```

### Omnigate — Intel iGPU (Experimental)

All-in-one image bundling LM Gate and Ollama with Intel iGPU acceleration via oneAPI. Requires `/dev/dri` device access on the host.

```bash
cp .env.sample .env   # edit with your values
docker compose -f docker/docker-compose.build.omni.intel.yml up -d
```

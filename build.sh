#!/bin/bash
set -e

# 总入口：无参数或 release|test|debug 按本机编译主包；pack / protenix / pxdesign / all 进入对应阶段脚本。

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    cat <<'EOF'
Usage: bash ./build.sh [release|test|debug]
default: release

Builds the main package for this machine (uname). Does not set GOOS=linux.
bioBox.exe is produced only when this script runs on Windows.

Stage scripts:
  bash ./build.sh pack [release|test|debug] [linux_amd64|linux_arm64|both|*.yaml]
  bash ./build.sh protenix
  bash ./build.sh pxdesign
  bash ./build.sh all [release|test|debug] [linux_amd64|linux_arm64|both|*.yaml]
EOF
    exit 0
fi

case "${1:-}" in
    pack)
        shift
        exec bash ./scripts/linux_release_pack.sh "$@"
        ;;
    protenix)
        shift
        exec bash ./scripts/suites/protenix/install_prefix_protenix.sh "$@"
        ;;
    pxdesign)
        shift
        exec bash ./scripts/suites/pxdesign/install_prefix_pxdesign.sh "$@"
        ;;
    all)
        shift
        bash ./scripts/linux_release_pack.sh "$@"
        bash ./scripts/suites/protenix/install_prefix_protenix.sh
        bash ./scripts/suites/pxdesign/install_prefix_pxdesign.sh
        exit 0
        ;;
    linux_amd64|linux_arm64|both|*.yaml|*.yml)
        exec bash ./scripts/linux_release_pack.sh "$@"
        ;;
esac

product_name=$(sed -nE 's/^[[:space:]]*ProjectName[[:space:]]*=[[:space:]]*"([^"]+)".*/\1/p' ./bb_config/config.go | head -n1)
Product_version_key="ProjectVersion"
VersionFile=./bb_config/config.go
CURRENT_VERSION=$(sed -nE "s/^[[:space:]]*${Product_version_key}[[:space:]]*=[[:space:]]*\"([^\"]+)\".*/\\1/p" "$VersionFile" | head -n1)

if [[ -z "$product_name" || -z "$CURRENT_VERSION" ]]; then
    echo "failed to read ProjectName or ProjectVersion from ${VersionFile}" >&2
    exit 1
fi

build_path=./build
RUN_MODE=release

UPLOAD_TMP_DIR=upload_tmp_dir

extend_app_names=()

OS_TYPE="Unknown"
GetOSType(){
    uNames=`uname -s`
    osName=${uNames: 0: 4}
    if [ "$osName" == "Darw" ] # Darwin
    then
        OS_TYPE="Darwin"
    elif [ "$osName" == "Linu" ] # Linux
    then
        OS_TYPE="Linux"
    elif [ "$osName" == "MING" ] # MINGW, windows, git-bash
    then
        OS_TYPE="Windows"
    else
        OS_TYPE="Unknown"
    fi
}
GetOSType

function toBuild() {

    rm -rf ${build_path}/${RUN_MODE}
    mkdir -p ${build_path}/${RUN_MODE}

    go_version=$(go version | awk '{print $3}')
    commit_hash=$(git show -s --format=%H)
    commit_date=$(git show -s --format="%ci")

    if [[ "$OS_TYPE" == "Darwin" ]]; then
        # macOS
        formatted_time=$(date -u -j -f "%Y-%m-%d %H:%M:%S %z" "${commit_date}" "+%Y-%m-%d_%H:%M:%S")
    else
        # Linux
        formatted_time=$(date -u -d "${commit_date}" "+%Y-%m-%d_%H:%M:%S")
    fi

    build_time=$(date +"%Y-%m-%d_%H:%M:%S")

    ld_flag_master="-X main.mGitCommitHash=${commit_hash} -X main.mGitCommitTime=${formatted_time} -X main.mGoVersion=${go_version} -X main.mPackageOS=${OS_TYPE} -X main.mPackageTime=${build_time} -X main.mRunMode=${RUN_MODE} -s -w"

    bin_name="${product_name}"
    if [[ "$OS_TYPE" == "Windows" ]]; then
        bin_name="${product_name}.exe"
    fi

    echo "build ${product_name} ${OS_TYPE} ${RUN_MODE}"
    go build -o "${build_path}/${RUN_MODE}/${product_name}/${bin_name}" -trimpath -ldflags "${ld_flag_master}" .
    chmod a+x "${build_path}/${RUN_MODE}/${product_name}/${bin_name}"
    [[ -f "./example_files/${product_name}.service" ]] && cp "./example_files/${product_name}.service" "${build_path}/${RUN_MODE}/${product_name}/"
    [[ -f "./example_files/install_${product_name}.sh" ]] && cp "./example_files/install_${product_name}.sh" "${build_path}/${RUN_MODE}/${product_name}/install_${product_name}.sh"
    mkdir -p "${build_path}/${RUN_MODE}/${product_name}/conf"
    [[ -f "./example_files/config_example.yaml" ]] && cp "./example_files/config_example.yaml" "${build_path}/${RUN_MODE}/${product_name}/conf/config.yaml"


    for extend_app in "${extend_app_names[@]}"; do
        go build -o ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/${product_name}_${extend_app} -trimpath -ldflags "${ld_flag_master}" ./${product_name}_${extend_app}/main.go \
        && chmod a+x ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/${product_name}_${extend_app} \
        && cp ./example_files/${product_name}_${extend_app}.service ${build_path}/${RUN_MODE}/${product_name}_${extend_app} \
        && cp ./example_files/install_${product_name}_${extend_app}.sh ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/install_${product_name}_${extend_app}.sh \
        && mkdir -p ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/conf \
        && cp ./example_files/config_${extend_app}_example.yaml ${build_path}/${RUN_MODE}/${product_name}_${extend_app}/conf/config_${extend_app}.yaml
    done

    package_files
}

function package_files(){
    local OS_TYPE_LOWER=$(echo "${OS_TYPE}" | awk '{print tolower($0)}')
    cd ${build_path}/${RUN_MODE} \
    && if [[ "$OS_TYPE" == "Windows" ]]; then
            7z a ./${product_name}_${OS_TYPE_LOWER}_${RUN_MODE}_${CURRENT_VERSION}.zip ./${product_name} >/dev/null 2>&1
        else
            zip -r ./${product_name}_${OS_TYPE_LOWER}_${RUN_MODE}_${CURRENT_VERSION}.zip ./${product_name}
            for extend_app in "${extend_app_names[@]}"; do
                zip -r ./${product_name}_${extend_app}_${OS_TYPE_LOWER}_${RUN_MODE}_${CURRENT_VERSION}.zip ./${product_name}_${extend_app}
            done
        fi \
    && mkdir -p ../${UPLOAD_TMP_DIR} \
    && mv *.zip ../${UPLOAD_TMP_DIR} \
    && cd ../
}


function handlerunMode() {
    if [[ "$1" == "release" || "$1" == "" ]]; then
        RUN_MODE=release
    elif [[ "$1" == "test" ]]; then
        RUN_MODE=test
    elif [[ "$1" == "debug" ]]; then
        RUN_MODE=debug
    else
        echo "Usage: bash ./build.sh [release|test|debug], default: release"
        echo "Stages: bash ./build.sh pack | protenix | pxdesign | all"
        exit 0
    fi
}


# 前端与 test_ui 随打包构建；缺 Bun 时失败，dist 不进仓。
handlerunMode "$1" && ./gen_test_ui.sh && ./gen_frontend.sh && toBuild

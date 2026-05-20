/* smartmon_config.h – cross-platform substitute for the autoconf-generated
 * version that is normally produced by running ./configure in the
 * smartmontools source tree.
 *
 * The dianlight/smartmontools-sdk release archives omit this file because it
 * is build-system artefact. This hand-crafted version covers the four macros
 * the SDK public headers actually test, derived from well-known compiler and
 * OS predefined macros so that no ./configure step is required.
 */

#pragma once

/* __attribute__((packed)) is available on GCC and Clang. */
#if defined(__GNUC__) || defined(__clang__)
#  define SMARTMON_HAVE_ATTR_PACKED 1
#endif

/* __int128 is available on 64-bit GCC/Clang targets. */
#if defined(__SIZEOF_INT128__)
#  define SMARTMON_HAVE___INT128 1
#endif

/* long double is wider than double only on x86/x86-64 (80-bit extended
 * precision).  On ARM64 and other LP64 platforms long double == double. */
#if defined(__i386__) || defined(__x86_64__)
#  define SMARTMON_HAVE_LONG_DOUBLE_WIDER 1
#endif

/* <byteswap.h> is a glibc/Linux extension; it is not available on macOS. */
#if defined(__linux__)
#  define HAVE_BYTESWAP_H 1
#endif

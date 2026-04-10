#include "solock-desktop.h"
#include <string.h>

void solock_secure_free(char *secret)
{
    if (!secret) return;
    explicit_bzero(secret, strlen(secret));
    g_free(secret);
}

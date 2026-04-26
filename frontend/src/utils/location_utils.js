/* utils for getting user's location */

export const getLocation = () => {
  return fetch("https://geolocation-db.com/json/", {
    method: "GET",
  }).then((res) => res.json())
}

const getCurrentPositionWithOptions = (options) => {
  return new Promise((resolve, reject) => {
    navigator.geolocation.getCurrentPosition(resolve, reject, options)
  })
}

const mapGeolocationError = (error) => {
  if (!error) return "Could not access your location."
  if (error.code === 1) {
    return "Location permission denied. Please allow location in browser/site settings."
  }
  if (error.code === 2) {
    return "Location unavailable right now. Please check your network/GPS and retry."
  }
  if (error.code === 3) {
    return "Location request timed out. Try again in a stronger signal area."
  }
  return error.message || "Could not access your location."
}

export const getCurrentBrowserLocation = async () => {
  if (!navigator.geolocation) {
    throw new Error("Geolocation is not supported by this browser")
  }

  try {
    const position = await getCurrentPositionWithOptions({
      enableHighAccuracy: true,
      timeout: 12000,
      maximumAge: 30000,
    })
    return {
      latitude: position.coords.latitude,
      longitude: position.coords.longitude,
      accuracyMeters: position.coords.accuracy,
    }
  } catch (firstError) {
    // Retry with less strict settings. This often succeeds on devices where
    // high-accuracy GPS locks are slow/unavailable.
    try {
      const position = await getCurrentPositionWithOptions({
        enableHighAccuracy: false,
        timeout: 25000,
        maximumAge: 120000,
      })
      return {
        latitude: position.coords.latitude,
        longitude: position.coords.longitude,
        accuracyMeters: position.coords.accuracy,
      }
    } catch (secondError) {
      // Final fallback: coarse IP-based location so downstream features
      // can still run even when device geolocation is unavailable.
      try {
        const ipLoc = await getLocation()
        const latitude = Number(ipLoc?.latitude)
        const longitude = Number(ipLoc?.longitude)
        if (!Number.isNaN(latitude) && !Number.isNaN(longitude)) {
          return {
            latitude,
            longitude,
            accuracyMeters: null,
          }
        }
      } catch (ipErr) {
        // Ignore and surface the original geolocation issue below.
      }

      throw new Error(mapGeolocationError(secondError || firstError))
    }
  }
}
